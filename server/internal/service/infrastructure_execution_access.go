package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/infrastructureexec"
	"github.com/multica-ai/multica/server/internal/integrations/transwarp"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

var ErrInfrastructureConflict = errors.New("infrastructure deployment changed")

type ExecutionAccessInput struct {
	infrastructureexec.Access
	AuthType     string `json:"auth_type"`
	ProbeHostKey bool   `json:"probe_host_key"`
}

type ExecutionAccessInfo struct {
	Configured                  bool                                   `json:"configured"`
	Host                        string                                 `json:"host"`
	Port                        int                                    `json:"port"`
	Username                    string                                 `json:"username"`
	HostKey                     string                                 `json:"host_key"`
	Context                     string                                 `json:"context"`
	AuthType                    string                                 `json:"auth_type"`
	RequiresContextSelection    bool                                   `json:"requires_context_selection"`
	ContextOptions              []infrastructureexec.KubernetesContext `json:"context_options"`
	RequiresHostKeyConfirmation bool                                   `json:"requires_host_key_confirmation"`
	HostKeyFingerprint          string                                 `json:"host_key_fingerprint"`
	PreviousHostKeyFingerprint  string                                 `json:"previous_host_key_fingerprint"`
}

type executionAccessRecord struct {
	Access     infrastructureexec.Access `json:"access"`
	ManagerURL string                    `json:"manager_url"`
}

func (s *InfrastructureService) executionAccessRecords(row db.InfrastructureConnection) (map[string]executionAccessRecord, error) {
	result := map[string]executionAccessRecord{}
	if len(row.ExecutionAccessCiphertext) == 0 {
		return result, nil
	}
	if s.Secrets == nil {
		return nil, errors.New("infrastructure secret encryption is not configured")
	}
	plain, err := s.Secrets.Open(row.ExecutionAccessCiphertext)
	if err != nil || json.Unmarshal(plain, &result) != nil {
		return nil, errors.New("could not decrypt infrastructure execution access")
	}
	return result, nil
}

func (s *InfrastructureService) executionAccess(row db.InfrastructureConnection, cluster int64) (infrastructureexec.Access, bool, error) {
	records, err := s.executionAccessRecords(row)
	if err != nil {
		return infrastructureexec.Access{}, false, err
	}
	record, ok := records[strconv.FormatInt(cluster, 10)]
	return record.Access, ok && record.ManagerURL == row.BaseUrl, nil
}

func accessInfo(access infrastructureexec.Access, configured bool) ExecutionAccessInfo {
	method := "password"
	if access.PrivateKey != "" {
		method = "key"
	}
	if access.Port == 0 {
		access.Port = 22
	}
	return ExecutionAccessInfo{Configured: configured, Host: access.Host, Port: access.Port, Username: access.Username, HostKey: access.HostKey, Context: access.Context, AuthType: method, HostKeyFingerprint: infrastructureexec.HostKeyFingerprint(access.HostKey)}
}

func (s *InfrastructureService) GetExecutionAccess(ctx context.Context, workspace, connection pgtype.UUID, cluster int64) (ExecutionAccessInfo, error) {
	row, err := s.Queries.GetInfrastructureConnection(ctx, db.GetInfrastructureConnectionParams{ID: connection, WorkspaceID: workspace})
	if err != nil {
		return ExecutionAccessInfo{}, err
	}
	access, configured, err := s.executionAccess(row, cluster)
	if !configured {
		access = infrastructureexec.Access{}
	}
	return accessInfo(access, configured), err
}

func (s *InfrastructureService) dialExecution(ctx context.Context, access infrastructureexec.Access) (infrastructureexec.Runner, error) {
	if s.ExecutionDial != nil {
		return s.ExecutionDial(ctx, access)
	}
	return infrastructureexec.Dial(ctx, access)
}

func (s *InfrastructureService) SaveExecutionAccess(ctx context.Context, workspace, connection pgtype.UUID, cluster int64, input ExecutionAccessInput) (ExecutionAccessInfo, error) {
	if s.Secrets == nil {
		return ExecutionAccessInfo{}, errors.New("configure MULTICA_INFRASTRUCTURE_SECRET_KEY before saving execution credentials")
	}
	if cluster <= 0 {
		return ExecutionAccessInfo{}, errors.New("cluster_id is required")
	}
	row, err := s.Queries.GetInfrastructureConnection(ctx, db.GetInfrastructureConnectionParams{ID: connection, WorkspaceID: workspace})
	if err != nil {
		return ExecutionAccessInfo{}, err
	}
	client, err := s.client(row)
	if err != nil {
		return ExecutionAccessInfo{}, err
	}
	clusters, err := client.Clusters(ctx)
	if err != nil {
		return ExecutionAccessInfo{}, err
	}
	found := false
	for _, candidate := range clusters {
		found = found || candidate.ID == cluster
	}
	if !found {
		return ExecutionAccessInfo{}, errors.New("cluster does not belong to this Manager")
	}
	records, err := s.executionAccessRecords(row)
	if err != nil {
		return ExecutionAccessInfo{}, err
	}
	previous := records[strconv.FormatInt(cluster, 10)]
	input.Host, input.Username, input.Context, input.HostKey = strings.TrimSpace(input.Host), strings.TrimSpace(input.Username), strings.TrimSpace(input.Context), strings.TrimSpace(input.HostKey)
	if input.Port == 0 {
		input.Port = 22
	}
	sameHost := previous.ManagerURL == row.BaseUrl && previous.Access.Host == input.Host && previous.Access.Port == input.Port
	sameUser := sameHost && previous.Access.Username == input.Username
	switch input.AuthType {
	case "key":
		input.Password = ""
		if input.PrivateKey == "" && sameUser {
			input.PrivateKey = previous.Access.PrivateKey
		}
	case "password":
		input.PrivateKey = ""
		if input.Password == "" && sameUser {
			input.Password = previous.Access.Password
		}
	default:
		return ExecutionAccessInfo{}, errors.New("auth_type must be key or password")
	}
	if err := input.Access.ValidateCredentials(); err != nil {
		return ExecutionAccessInfo{}, err
	}
	checkCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	trustedKey := ""
	if sameHost {
		trustedKey = previous.Access.HostKey
	}
	resolved, review, err := infrastructureexec.PrepareHostKey(checkCtx, input.Access, trustedKey, input.ProbeHostKey)
	if err != nil {
		return ExecutionAccessInfo{}, err
	}
	if review != nil {
		info := accessInfo(input.Access, false)
		info.RequiresHostKeyConfirmation = true
		info.HostKey, info.HostKeyFingerprint, info.PreviousHostKeyFingerprint = review.Key, review.Fingerprint, review.PreviousFingerprint
		return info, nil
	}
	input.Access = resolved
	runner, err := s.dialExecution(checkCtx, input.Access)
	if err != nil {
		return ExecutionAccessInfo{}, err
	}
	defer runner.Close()
	selected, options, err := infrastructureexec.SelectContext(checkCtx, runner, input.Context)
	if err != nil {
		return ExecutionAccessInfo{}, err
	}
	if len(options) > 1 {
		info := accessInfo(input.Access, false)
		info.RequiresContextSelection, info.ContextOptions = true, options
		return info, nil
	}
	input.Context = selected
	if _, err := runner.Run(checkCtx, infrastructureexec.Kubectl(input.Context, "--request-timeout=15s", "get", "--raw=/version")...); err != nil {
		return ExecutionAccessInfo{}, fmt.Errorf("Kubernetes API check failed on execution node (SSH user %q): %w", input.Username, err)
	}
	if _, err := runner.Run(checkCtx, "docker", "version", "--format", "{{.Client.Version}}"); err != nil {
		return ExecutionAccessInfo{}, fmt.Errorf("verify Docker client: %w", err)
	}
	records[strconv.FormatInt(cluster, 10)] = executionAccessRecord{Access: input.Access, ManagerURL: row.BaseUrl}
	plain, _ := json.Marshal(records)
	sealed, err := s.Secrets.Seal(plain)
	if err != nil {
		return ExecutionAccessInfo{}, err
	}
	count, err := s.Queries.SaveInfrastructureExecutionAccess(ctx, db.SaveInfrastructureExecutionAccessParams{ID: connection, WorkspaceID: workspace, ExecutionAccessCiphertext: sealed, UpdatedAt: row.UpdatedAt})
	if err != nil {
		return ExecutionAccessInfo{}, err
	}
	if count != 1 {
		return ExecutionAccessInfo{}, fmt.Errorf("%w: Manager settings changed; reload before saving", ErrInfrastructureConflict)
	}
	return accessInfo(input.Access, true), nil
}

func requireImageService(ctx context.Context, client *transwarp.Client, cluster, service int64) error {
	if cluster <= 0 || service <= 0 {
		return errors.New("cluster_id and service_id are required")
	}
	resource, err := client.Service(ctx, service)
	if err != nil {
		return err
	}
	if resource.ClusterID != cluster {
		return errors.New("service does not belong to the selected cluster")
	}
	return nil
}

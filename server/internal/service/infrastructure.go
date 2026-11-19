package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/infrastructureexec"
	"github.com/multica-ai/multica/server/internal/integrations/transwarp"
	"github.com/multica-ai/multica/server/internal/util/secretbox"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type InfrastructureGrant struct {
	ClusterID int64 `json:"cluster_id"`
	ServiceID int64 `json:"service_id"`
	RoleID    int64 `json:"role_id"`
}

type InfrastructureConnection struct {
	ID            string                `json:"id"`
	Name          string                `json:"name"`
	URL           string                `json:"url"`
	Username      string                `json:"username"`
	CAPEM         string                `json:"ca_pem"`
	SkipTLSVerify bool                  `json:"skip_tls_verify"`
	RestartGrants []InfrastructureGrant `json:"restart_grants"`
}

type InfrastructureConnectionInput struct {
	Name          string `json:"name"`
	URL           string `json:"url"`
	Username      string `json:"username"`
	Password      string `json:"password"`
	CAPEM         string `json:"ca_pem"`
	SkipTLSVerify bool   `json:"skip_tls_verify"`
}

type InfrastructureInput struct {
	ConnectionID string           `json:"connection_id"`
	ClusterID    int64            `json:"cluster_id"`
	ServiceID    int64            `json:"service_id"`
	NodeID       int64            `json:"node_id"`
	RoleID       int64            `json:"role_id"`
	Hostname     string           `json:"hostname"`
	Level        string           `json:"level"`
	Keywords     string           `json:"keywords"`
	MetricNames  string           `json:"metric_names"`
	Start        int64            `json:"start"`
	End          int64            `json:"end"`
	Reason       string           `json:"reason"`
	OperationID  string           `json:"operation_id"`
	Action       string           `json:"action"`
	Items        []ImageSelection `json:"items"`
	Key          string           `json:"key"`
	Content      *string          `json:"content"`
	TaskID       string           `json:"-"`
}

type InfrastructureOperation struct {
	ID           string          `json:"id"`
	ConnectionID string          `json:"connection_id"`
	ClusterID    int64           `json:"cluster_id"`
	ServiceID    int64           `json:"service_id"`
	RoleID       int64           `json:"role_id"`
	ActorType    string          `json:"actor_type"`
	ActorID      string          `json:"actor_id"`
	Reason       string          `json:"reason"`
	JobID        *int64          `json:"job_id"`
	Status       string          `json:"status"`
	Error        string          `json:"error"`
	CreatedAt    string          `json:"created_at"`
	Kind         string          `json:"kind"`
	Details      json.RawMessage `json:"details"`
}

type infrastructureClientEntry struct {
	revision time.Time
	client   *transwarp.Client
}

type InfrastructureService struct {
	Queries       *db.Queries
	TxStarter     TxStarter
	Secrets       *secretbox.Box
	ExecutionDial infrastructureexec.Dialer
	clients       sync.Map
}

func infrastructureConnection(row db.InfrastructureConnection) InfrastructureConnection {
	grants := []InfrastructureGrant{}
	_ = json.Unmarshal(row.RestartGrants, &grants)
	return InfrastructureConnection{ID: uuidString(row.ID), Name: row.Name, URL: row.BaseUrl, Username: row.Username, CAPEM: row.CaPem, SkipTLSVerify: row.SkipTlsVerify, RestartGrants: grants}
}

func infrastructureOperation(row db.InfrastructureOperation) InfrastructureOperation {
	var jobID *int64
	if row.UpstreamJobID.Valid {
		value := row.UpstreamJobID.Int64
		jobID = &value
	}
	return InfrastructureOperation{ID: uuidString(row.ID), ConnectionID: uuidString(row.ConnectionID), ClusterID: row.ClusterID, ServiceID: row.ServiceID, RoleID: row.RoleID, ActorType: row.ActorType, ActorID: uuidString(row.ActorID), Reason: row.Reason, JobID: jobID, Status: row.Status, Error: row.Error, CreatedAt: row.CreatedAt.Time.UTC().Format(time.RFC3339), Kind: row.Kind, Details: json.RawMessage(row.Details)}
}

func (s *InfrastructureService) List(ctx context.Context, workspaceID pgtype.UUID) ([]InfrastructureConnection, error) {
	rows, err := s.Queries.ListInfrastructureConnections(ctx, workspaceID)
	result := make([]InfrastructureConnection, 0, len(rows))
	for _, row := range rows {
		result = append(result, infrastructureConnection(row))
	}
	return result, err
}

func (s *InfrastructureService) client(row db.InfrastructureConnection) (*transwarp.Client, error) {
	if s.Secrets == nil {
		return nil, errors.New("infrastructure secret encryption is not configured")
	}
	key := uuidString(row.ID)
	if cached, ok := s.clients.Load(key); ok {
		entry := cached.(infrastructureClientEntry)
		if entry.revision.Equal(row.UpdatedAt.Time) {
			return entry.client, nil
		}
	}
	password, err := s.Secrets.Open(row.PasswordCiphertext)
	if err != nil {
		return nil, errors.New("could not decrypt Manager credentials")
	}
	client, err := transwarp.New(transwarp.Config{URL: row.BaseUrl, Username: row.Username, Password: string(password), CAPEM: row.CaPem, SkipTLSVerify: row.SkipTlsVerify})
	if err != nil {
		return nil, err
	}
	s.clients.Store(key, infrastructureClientEntry{row.UpdatedAt.Time, client})
	return client, nil
}

func (s *InfrastructureService) Save(ctx context.Context, workspaceID, id, actorID pgtype.UUID, input InfrastructureConnectionInput) (InfrastructureConnection, error) {
	if s.Secrets == nil {
		return InfrastructureConnection{}, errors.New("infrastructure secret encryption is not configured")
	}
	input.Name = strings.TrimSpace(input.Name)
	input.URL = strings.TrimRight(strings.TrimSpace(input.URL), "/")
	input.Username = strings.TrimSpace(input.Username)
	if input.Name == "" || len(input.Name) > 120 {
		return InfrastructureConnection{}, errors.New("Manager name must contain 1–120 bytes")
	}
	if id.Valid && input.Password == "" {
		previous, err := s.Queries.GetInfrastructureConnection(ctx, db.GetInfrastructureConnectionParams{ID: id, WorkspaceID: workspaceID})
		if err != nil {
			return InfrastructureConnection{}, err
		}
		password, err := s.Secrets.Open(previous.PasswordCiphertext)
		if err != nil {
			return InfrastructureConnection{}, err
		}
		input.Password = string(password)
	}
	client, err := transwarp.New(transwarp.Config{URL: input.URL, Username: input.Username, Password: input.Password, CAPEM: input.CAPEM, SkipTLSVerify: input.SkipTLSVerify})
	if err != nil {
		return InfrastructureConnection{}, err
	}
	if _, err = client.Clusters(ctx); err != nil {
		return InfrastructureConnection{}, err
	}
	sealed, err := s.Secrets.Seal([]byte(input.Password))
	if err != nil {
		return InfrastructureConnection{}, err
	}
	var row db.InfrastructureConnection
	if id.Valid {
		row, err = s.Queries.UpdateInfrastructureConnection(ctx, db.UpdateInfrastructureConnectionParams{ID: id, WorkspaceID: workspaceID, Name: input.Name, BaseUrl: input.URL, Username: input.Username, PasswordCiphertext: sealed, CaPem: input.CAPEM, SkipTlsVerify: input.SkipTLSVerify})
	} else {
		row, err = s.Queries.CreateInfrastructureConnection(ctx, db.CreateInfrastructureConnectionParams{WorkspaceID: workspaceID, Name: input.Name, BaseUrl: input.URL, Username: input.Username, PasswordCiphertext: sealed, CaPem: input.CAPEM, CreatedBy: actorID, SkipTlsVerify: input.SkipTLSVerify})
	}
	if err != nil {
		return InfrastructureConnection{}, err
	}
	s.clients.Store(uuidString(row.ID), infrastructureClientEntry{row.UpdatedAt.Time, client})
	return infrastructureConnection(row), nil
}

func (s *InfrastructureService) Delete(ctx context.Context, workspaceID, id pgtype.UUID) error {
	tx, err := s.TxStarter.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	queries := s.Queries.WithTx(tx)
	if err := queries.DeleteInfrastructureOperations(ctx, db.DeleteInfrastructureOperationsParams{ConnectionID: id, WorkspaceID: workspaceID}); err != nil {
		return err
	}
	if err := queries.DeleteInfrastructureConnection(ctx, db.DeleteInfrastructureConnectionParams{ID: id, WorkspaceID: workspaceID}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.clients.Delete(uuidString(id))
	return nil
}

func (s *InfrastructureService) SetGrants(ctx context.Context, workspaceID, id pgtype.UUID, grants []InfrastructureGrant) (InfrastructureConnection, error) {
	if len(grants) > 256 {
		return InfrastructureConnection{}, errors.New("at most 256 restart grants may be configured")
	}
	row, err := s.Queries.GetInfrastructureConnection(ctx, db.GetInfrastructureConnectionParams{ID: id, WorkspaceID: workspaceID})
	if err != nil {
		return InfrastructureConnection{}, err
	}
	client, err := s.client(row)
	if err != nil {
		return InfrastructureConnection{}, err
	}
	seen := map[int64]bool{}
	for _, grant := range grants {
		if grant.ClusterID <= 0 || grant.ServiceID <= 0 || grant.RoleID <= 0 || seen[grant.RoleID] {
			return InfrastructureConnection{}, errors.New("each restart grant must identify a distinct cluster, service and role")
		}
		role, err := client.Role(ctx, grant.RoleID)
		if err != nil {
			return InfrastructureConnection{}, err
		}
		if !grantMatchesRole(grant, role) {
			return InfrastructureConnection{}, errors.New("restart grant does not match the Manager role")
		}
		seen[grant.RoleID] = true
	}
	if grants == nil {
		grants = []InfrastructureGrant{}
	}
	encoded, err := json.Marshal(grants)
	if err != nil {
		return InfrastructureConnection{}, err
	}
	row, err = s.Queries.UpdateInfrastructureGrants(ctx, db.UpdateInfrastructureGrantsParams{ID: id, WorkspaceID: workspaceID, RestartGrants: encoded})
	return infrastructureConnection(row), err
}

func grantMatchesRole(grant InfrastructureGrant, role transwarp.Role) bool {
	return role.ID == grant.RoleID && role.ServiceID == grant.ServiceID && role.Node.ClusterID == grant.ClusterID && role.Installed
}

func (s *InfrastructureService) Execute(ctx context.Context, workspaceID, connectionID, actorID pgtype.UUID, actorType, tool string, input InfrastructureInput) (any, error) {
	row, err := s.Queries.GetInfrastructureConnection(ctx, db.GetInfrastructureConnectionParams{ID: connectionID, WorkspaceID: workspaceID})
	if err != nil {
		return nil, err
	}
	client, err := s.client(row)
	if err != nil {
		return nil, err
	}
	switch tool {
	case "list_clusters":
		return client.Clusters(ctx)
	case "list_nodes", "list_services":
		if input.ClusterID <= 0 {
			return nil, errors.New("cluster_id is required")
		}
		if tool == "list_nodes" {
			return client.Nodes(ctx, input.ClusterID)
		}
		return client.Services(ctx, input.ClusterID)
	case "service_relations":
		if input.ServiceID <= 0 {
			return nil, errors.New("service_id is required")
		}
		return client.Relations(ctx, input.ServiceID)
	case "list_roles":
		if input.ServiceID <= 0 {
			return nil, errors.New("service_id is required")
		}
		return client.Roles(ctx, input.ServiceID)
	case "list_metainfo_files":
		return client.ServiceMetaFiles(ctx, input.ServiceID)
	case "read_metainfo_file":
		return client.ServiceMetaFile(ctx, input.ServiceID, input.Key)
	case "save_metainfo_file":
		return s.saveMetainfo(ctx, row, client, actorID, actorType, input)
	case "node_metrics":
		if input.NodeID <= 0 || strings.TrimSpace(input.MetricNames) == "" {
			return nil, errors.New("node_id and metric_names are required")
		}
		return client.NodeMetrics(ctx, input.NodeID, input.MetricNames)
	case "search_logs":
		if input.ServiceID <= 0 || input.Hostname == "" {
			return nil, errors.New("service_id and hostname are required")
		}
		switch input.Level {
		case "FATAL", "ERROR", "WARN", "INFO", "DEBUG", "TRACE":
		default:
			return nil, errors.New("a valid log level is required")
		}
		if input.End == 0 {
			input.End = time.Now().UnixMilli()
		}
		if input.Start == 0 {
			input.Start = input.End - 15*60*1000
		}
		if input.Start < 0 || input.End < input.Start || input.End-input.Start > 24*60*60*1000 {
			return nil, errors.New("log searches must cover at most 24 hours")
		}
		return client.SearchLogs(ctx, input.ServiceID, input.Hostname, input.Level, input.Keywords, input.Start, input.End)
	case "restart_role":
		return s.restart(ctx, row, client, actorID, actorType, input)
	case "list_operations":
		rows, err := s.Queries.ListInfrastructureOperations(ctx, db.ListInfrastructureOperationsParams{WorkspaceID: workspaceID, ConnectionID: connectionID})
		if err != nil {
			return nil, err
		}
		result := make([]InfrastructureOperation, 0, len(rows))
		for _, operation := range rows {
			result = append(result, infrastructureOperation(operation))
		}
		return result, nil
	default:
		return nil, errors.New("unknown infrastructure tool")
	}
}

func (s *InfrastructureService) ServiceMetaClient(ctx context.Context, workspaceID, connectionID pgtype.UUID) (*transwarp.Client, error) {
	row, err := s.Queries.GetInfrastructureConnection(ctx, db.GetInfrastructureConnectionParams{ID: connectionID, WorkspaceID: workspaceID})
	if err != nil {
		return nil, err
	}
	return s.client(row)
}

func (s *InfrastructureService) restart(ctx context.Context, row db.InfrastructureConnection, client *transwarp.Client, actorID pgtype.UUID, actorType string, input InfrastructureInput) (InfrastructureOperation, error) {
	grant := InfrastructureGrant{input.ClusterID, input.ServiceID, input.RoleID}
	allowed := false
	for _, candidate := range infrastructureConnection(row).RestartGrants {
		if candidate == grant {
			allowed = true
			break
		}
	}
	if !allowed {
		return InfrastructureOperation{}, errors.New("automatic restart is not authorized for this Manager, cluster, service and role")
	}
	input.Reason = strings.TrimSpace(input.Reason)
	if input.Reason == "" || len(input.Reason) > 2000 {
		return InfrastructureOperation{}, errors.New("a restart reason of at most 2000 bytes is required")
	}
	role, err := client.Role(ctx, input.RoleID)
	if err != nil {
		return InfrastructureOperation{}, err
	}
	if !grantMatchesRole(grant, role) {
		return InfrastructureOperation{}, errors.New("role ownership changed; review its restart authorization")
	}
	operation, err := s.Queries.CreateInfrastructureOperation(ctx, db.CreateInfrastructureOperationParams{WorkspaceID: row.WorkspaceID, ConnectionID: row.ID, ClusterID: input.ClusterID, ServiceID: input.ServiceID, RoleID: input.RoleID, ActorType: actorType, ActorID: actorID, Reason: input.Reason})
	if err != nil {
		return InfrastructureOperation{}, err
	}
	job, callErr := client.RestartRole(ctx, input.RoleID)
	status, errorText := "submitted", ""
	if callErr != nil {
		status = "unknown"
		if errors.Is(callErr, transwarp.ErrPrecheck) {
			status = "failed"
		}
		errorText = callErr.Error()
	}
	// A network failure can follow submission. Keep the record and never retry
	// a state-changing Manager request automatically.
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	operation, err = s.Queries.UpdateInfrastructureOperation(writeCtx, db.UpdateInfrastructureOperationParams{ID: operation.ID, WorkspaceID: row.WorkspaceID, UpstreamJobID: pgtype.Int8{Int64: job.ID, Valid: job.ID > 0}, Status: status, Error: errorText})
	if err != nil {
		return InfrastructureOperation{}, fmt.Errorf("Manager operation may have been submitted; audit update failed: %w", err)
	}
	return infrastructureOperation(operation), nil
}

func (s *InfrastructureService) RefreshOperation(ctx context.Context, workspaceID, connectionID, id pgtype.UUID) (InfrastructureOperation, error) {
	operation, err := s.Queries.GetInfrastructureOperation(ctx, db.GetInfrastructureOperationParams{ID: id, WorkspaceID: workspaceID, ConnectionID: connectionID})
	if err != nil {
		return InfrastructureOperation{}, err
	}
	if operation.Kind == "publish_images" || operation.Kind == "restart_pods" {
		return s.refreshImageOperation(ctx, operation)
	}
	if !operation.UpstreamJobID.Valid || operation.Status == "succeeded" || operation.Status == "failed" {
		return infrastructureOperation(operation), nil
	}
	row, err := s.Queries.GetInfrastructureConnection(ctx, db.GetInfrastructureConnectionParams{ID: connectionID, WorkspaceID: workspaceID})
	if err != nil {
		return InfrastructureOperation{}, err
	}
	client, err := s.client(row)
	if err != nil {
		return InfrastructureOperation{}, err
	}
	job, err := client.Job(ctx, operation.UpstreamJobID.Int64)
	if err != nil {
		return InfrastructureOperation{}, err
	}
	status := "running"
	switch job.Status {
	case "SUCCESSFUL":
		role, err := client.Role(ctx, operation.RoleID)
		if err != nil {
			return InfrastructureOperation{}, err
		}
		if role.Status == "RUNNING" && role.Health == "HEALTHY" {
			status = "succeeded"
		} else {
			status = "verification_required"
		}
	case "FAILED", "WARNING", "IGNORED_FAILED", "IGNORED_WARNING", "CANCELED":
		status = "failed"
	case "PREPARING", "WAITING", "RUNNING":
		status = "running"
	default:
		status = "unknown"
	}
	operation, err = s.Queries.UpdateInfrastructureOperation(ctx, db.UpdateInfrastructureOperationParams{ID: id, WorkspaceID: workspaceID, UpstreamJobID: operation.UpstreamJobID, Status: status, Error: ""})
	return infrastructureOperation(operation), err
}

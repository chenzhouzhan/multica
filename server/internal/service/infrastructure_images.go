package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/infrastructureexec"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type ImageResources struct {
	Configured bool                        `json:"configured"`
	Targets    []infrastructureexec.Target `json:"targets"`
}

type ImageSelection struct {
	Key    string `json:"key"`
	Source string `json:"source"`
}

type ImagePreviewInput struct {
	Action string           `json:"action"`
	Items  []ImageSelection `json:"items"`
	Reason string           `json:"reason"`
}

type ImageReleaseItem struct {
	Target            infrastructureexec.Target        `json:"target"`
	Image             infrastructureexec.ResolvedImage `json:"image"`
	Status            string                           `json:"status"`
	AppliedGeneration int64                            `json:"applied_generation"`
}

type ImageReleaseStep struct {
	Kind      string `json:"kind"`
	TargetKey string `json:"target_key"`
	Status    string `json:"status"`
	Error     string `json:"error"`
}

type ImageReleasePlan struct {
	TaskID           string             `json:"task_id,omitempty"`
	OriginatorUserID string             `json:"originator_user_id,omitempty"`
	ConfigRevision   string             `json:"config_revision"`
	Items            []ImageReleaseItem `json:"items"`
	Steps            []ImageReleaseStep `json:"steps"`
}

type ImageOperationActor struct {
	ID           pgtype.UUID
	Type         string
	TaskID       string
	OriginatorID pgtype.UUID
}

func (actor ImageOperationActor) validate() error {
	if !actor.ID.Valid || (actor.Type != "user" && actor.Type != "agent") {
		return errors.New("a valid image operation actor is required")
	}
	if actor.Type == "agent" {
		if _, err := util.ParseUUID(actor.TaskID); err != nil || !actor.OriginatorID.Valid {
			return errors.New("agent image operations require a task and human originator")
		}
	} else if actor.TaskID != "" {
		return errors.New("user image operations cannot use an agent task")
	}
	return nil
}

func imageActorMatches(operation db.InfrastructureOperation, plan ImageReleasePlan, actor ImageOperationActor) bool {
	if actor.validate() != nil || operation.ActorID != actor.ID || operation.ActorType != actor.Type || plan.TaskID != actor.TaskID {
		return false
	}
	return actor.Type != "agent" || plan.OriginatorUserID == uuidString(actor.OriginatorID)
}

func roleIdentity(value string) string {
	return strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(value))
}

func (s *InfrastructureService) imageResources(ctx context.Context, row db.InfrastructureConnection, cluster, service int64) (ImageResources, error) {
	client, err := s.client(row)
	if err != nil {
		return ImageResources{}, err
	}
	if err := requireImageService(ctx, client, cluster, service); err != nil {
		return ImageResources{}, err
	}
	access, configured, err := s.executionAccess(row, cluster)
	result := ImageResources{Configured: configured, Targets: []infrastructureexec.Target{}}
	if err != nil || !configured {
		return result, err
	}
	refs, err := client.ServiceWorkloads(ctx, service)
	if err != nil {
		return result, err
	}
	roles, err := client.Roles(ctx, service)
	if err != nil {
		return result, err
	}
	runner, err := s.dialExecution(ctx, access)
	if err != nil {
		return result, err
	}
	defer runner.Close()
	for _, ref := range refs {
		workload, err := infrastructureexec.ReadWorkload(ctx, runner, access.Context, ref.Kind, ref.Namespace, ref.Name)
		if err != nil {
			return result, err
		}
		if ref.UID != "" && ref.UID != workload.Metadata.UID {
			return result, fmt.Errorf("%w: Manager workload was recreated", ErrInfrastructureConflict)
		}
		for _, target := range workload.Targets() {
			// Only exact role/container identities are associated; workload names
			// are not heuristically matched across service instances.
			for _, role := range roles {
				if role.Type != "" && roleIdentity(role.Type) == roleIdentity(target.Container) {
					target.RoleType, target.RoleName = role.Type, role.Type
					break
				}
			}
			result.Targets = append(result.Targets, target)
		}
	}
	sort.Slice(result.Targets, func(i, j int) bool { return result.Targets[i].Key < result.Targets[j].Key })
	return result, nil
}

func (s *InfrastructureService) ImageResources(ctx context.Context, workspace, connection pgtype.UUID, cluster, service int64) (ImageResources, error) {
	row, err := s.Queries.GetInfrastructureConnection(ctx, db.GetInfrastructureConnectionParams{ID: connection, WorkspaceID: workspace})
	if err != nil {
		return ImageResources{}, err
	}
	return s.imageResources(ctx, row, cluster, service)
}

func (s *InfrastructureService) PreviewImages(ctx context.Context, workspace, connection pgtype.UUID, actor ImageOperationActor, cluster, service int64, input ImagePreviewInput) (InfrastructureOperation, error) {
	if err := actor.validate(); err != nil {
		return InfrastructureOperation{}, err
	}
	if input.Action != "publish_images" && input.Action != "restart_pods" {
		return InfrastructureOperation{}, errors.New("action must be publish_images or restart_pods")
	}
	if len(input.Items) == 0 || len(input.Items) > 16 || len(input.Reason) > 2000 {
		return InfrastructureOperation{}, errors.New("select 1–16 role containers and a reason of at most 2000 bytes")
	}
	plan := ImageReleasePlan{TaskID: actor.TaskID, Items: []ImageReleaseItem{}, Steps: []ImageReleaseStep{{Kind: "discover", Status: "pending"}}}
	if actor.Type == "agent" {
		plan.OriginatorUserID = uuidString(actor.OriginatorID)
	}
	seen := map[string]bool{}
	for _, item := range input.Items {
		if item.Key == "" || len(item.Key) > 1024 || seen[item.Key] {
			return InfrastructureOperation{}, errors.New("select distinct role containers")
		}
		seen[item.Key] = true
		item.Source = strings.TrimSpace(item.Source)
		if input.Action == "publish_images" {
			if _, err := infrastructureexec.ParseImage(item.Source); err != nil {
				return InfrastructureOperation{}, err
			}
		}
		plan.Items = append(plan.Items, ImageReleaseItem{Target: infrastructureexec.Target{Key: item.Key}, Image: infrastructureexec.ResolvedImage{Source: item.Source}, Status: "pending"})
	}
	row, err := s.Queries.GetInfrastructureConnection(ctx, db.GetInfrastructureConnectionParams{ID: connection, WorkspaceID: workspace})
	if err != nil {
		return InfrastructureOperation{}, err
	}
	client, err := s.client(row)
	if err != nil {
		return InfrastructureOperation{}, err
	}
	if err := requireImageService(ctx, client, cluster, service); err != nil {
		return InfrastructureOperation{}, err
	}
	_, configured, err := s.executionAccess(row, cluster)
	if err != nil {
		return InfrastructureOperation{}, err
	}
	if !configured {
		return InfrastructureOperation{}, errors.New("configure cluster execution access before publishing images")
	}
	plan.ConfigRevision = row.UpdatedAt.Time.UTC().Format(time.RFC3339Nano)
	details, _ := json.Marshal(plan)
	operation, err := s.Queries.CreateInfrastructureImageOperation(ctx, db.CreateInfrastructureImageOperationParams{WorkspaceID: workspace, ConnectionID: connection, ClusterID: cluster, ServiceID: service, ActorID: actor.ID, ActorType: actor.Type, Reason: strings.TrimSpace(input.Reason), Kind: input.Action, Details: details})
	if err != nil {
		return InfrastructureOperation{}, err
	}
	go s.prepareImageOperation(operation, row, plan)
	return infrastructureOperation(operation), nil
}

func (s *InfrastructureService) persistImageOperation(operation db.InfrastructureOperation, status, errorText string, plan ImageReleasePlan) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	details, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	_, err = s.Queries.UpdateInfrastructureImageOperation(ctx, db.UpdateInfrastructureImageOperationParams{ID: operation.ID, WorkspaceID: operation.WorkspaceID, Status: status, Error: errorText, Details: details})
	return err
}

func (s *InfrastructureService) prepareImageOperation(operation db.InfrastructureOperation, row db.InfrastructureConnection, plan ImageReleasePlan) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	fail := func(err error) {
		for index := range plan.Steps {
			if plan.Steps[index].Status == "running" {
				plan.Steps[index].Status, plan.Steps[index].Error = "failed", err.Error()
			}
		}
		_ = s.persistImageOperation(operation, "failed", err.Error(), plan)
	}
	plan.Steps[0].Status = "running"
	if err := s.persistImageOperation(operation, "preparing", "", plan); err != nil {
		return
	}
	resources, err := s.imageResources(ctx, row, operation.ClusterID, operation.ServiceID)
	if err != nil {
		fail(err)
		return
	}
	byKey := map[string]infrastructureexec.Target{}
	for _, target := range resources.Targets {
		byKey[target.Key] = target
	}
	for index := range plan.Items {
		item := &plan.Items[index]
		target, exists := byKey[item.Target.Key]
		if !exists {
			fail(errors.New("selected role container does not belong to this service; reload its current deployments"))
			return
		}
		item.Target = target
		if target.BlockedReason != "" {
			fail(errors.New(target.BlockedReason))
			return
		}
		if operation.Kind == "restart_pods" {
			item.Image.Source = target.Image
		}
		plan.Steps = append(plan.Steps, ImageReleaseStep{Kind: "resolve", TargetKey: target.Key, Status: "pending"})
	}
	plan.Steps[0].Status = "succeeded"
	access, _, err := s.executionAccess(row, operation.ClusterID)
	if err != nil {
		fail(err)
		return
	}
	runner, err := s.dialExecution(ctx, access)
	if err != nil {
		fail(err)
		return
	}
	defer runner.Close()
	for index := range plan.Items {
		plan.Steps[index+1].Status = "running"
		if err := s.persistImageOperation(operation, "preparing", "", plan); err != nil {
			return
		}
		image, err := infrastructureexec.ResolveImage(ctx, runner, plan.Items[index].Image.Source)
		if err != nil {
			plan.Items[index].Status = "failed"
			fail(err)
			return
		}
		plan.Items[index].Image, plan.Items[index].Status = image, "resolved"
		plan.Steps[index+1].Status = "succeeded"
	}
	_ = s.persistImageOperation(operation, "ready", "", plan)
}

func (s *InfrastructureService) StartImageOperation(ctx context.Context, workspace, connection pgtype.UUID, actor ImageOperationActor, id pgtype.UUID, cluster, service int64) (InfrastructureOperation, error) {
	operation, err := s.Queries.GetInfrastructureOperation(ctx, db.GetInfrastructureOperationParams{ID: id, WorkspaceID: workspace, ConnectionID: connection})
	if err != nil {
		return InfrastructureOperation{}, err
	}
	if operation.ClusterID != cluster || operation.ServiceID != service {
		return InfrastructureOperation{}, pgx.ErrNoRows
	}
	var plan ImageReleasePlan
	if json.Unmarshal(operation.Details, &plan) != nil || !imageActorMatches(operation, plan, actor) {
		return InfrastructureOperation{}, fmt.Errorf("%w: image preview belongs to another actor or task", ErrInfrastructureConflict)
	}
	operation, err = s.Queries.ClaimInfrastructureImageOperation(ctx, db.ClaimInfrastructureImageOperationParams{ID: id, WorkspaceID: workspace, ConnectionID: connection, ActorID: actor.ID, ActorType: actor.Type, TaskID: actor.TaskID})
	if errors.Is(err, pgx.ErrNoRows) {
		return InfrastructureOperation{}, fmt.Errorf("%w: preview expired, was already submitted, or belongs to another user; read and confirm again", ErrInfrastructureConflict)
	}
	if err != nil {
		return InfrastructureOperation{}, err
	}
	go s.runImageOperation(operation)
	return infrastructureOperation(operation), nil
}

func (s *InfrastructureService) refreshImageOperation(ctx context.Context, operation db.InfrastructureOperation) (InfrastructureOperation, error) {
	if operation.Status == "verification_required" {
		return s.reverifyImageOperation(ctx, operation)
	}
	status := operation.Status
	if (status == "preparing" || status == "running") && time.Since(operation.UpdatedAt.Time) > 35*time.Minute {
		status = "unknown"
	}
	if status == "ready" && time.Since(operation.UpdatedAt.Time) > 10*time.Minute {
		status = "expired"
	}
	if status != operation.Status {
		var plan ImageReleasePlan
		if err := json.Unmarshal(operation.Details, &plan); err != nil {
			return InfrastructureOperation{}, err
		}
		if err := s.persistImageOperation(operation, status, "preview or worker timed out; inspect the current deployment before submitting again", plan); err != nil {
			return InfrastructureOperation{}, err
		}
		operation.Status, operation.Error = status, "preview or worker timed out; inspect the current deployment before submitting again"
	}
	return infrastructureOperation(operation), nil
}

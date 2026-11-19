package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/multica-ai/multica/server/internal/infrastructureexec"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Verification only reads Kubernetes resources. It never resubmits a patch or restarts Pods.
func (s *InfrastructureService) reverifyImageOperation(ctx context.Context, operation db.InfrastructureOperation) (InfrastructureOperation, error) {
	var plan ImageReleasePlan
	if err := json.Unmarshal(operation.Details, &plan); err != nil {
		return InfrastructureOperation{}, err
	}
	if len(plan.Items) == 0 {
		return InfrastructureOperation{}, errors.New("invalid stored image preview")
	}
	row, err := s.Queries.GetInfrastructureConnection(ctx, db.GetInfrastructureConnectionParams{ID: operation.ConnectionID, WorkspaceID: operation.WorkspaceID})
	if err != nil {
		return InfrastructureOperation{}, err
	}
	if row.UpdatedAt.Time.UTC().Format(time.RFC3339Nano) != plan.ConfigRevision {
		return infrastructureOperation(operation), nil
	}
	resources, err := s.imageResources(ctx, row, operation.ClusterID, operation.ServiceID)
	if err != nil {
		return InfrastructureOperation{}, err
	}
	allowed := map[string]infrastructureexec.Target{}
	for _, target := range resources.Targets {
		allowed[target.Key] = target
	}
	access, configured, err := s.executionAccess(row, operation.ClusterID)
	if err != nil {
		return InfrastructureOperation{}, err
	}
	if !configured {
		return InfrastructureOperation{}, errors.New("cluster execution access is no longer configured")
	}
	runner, err := s.dialExecution(ctx, access)
	if err != nil {
		return InfrastructureOperation{}, err
	}
	defer runner.Close()
	for _, group := range imageGroups(plan) {
		first := plan.Items[group[0]]
		if first.Status != "verification_required" && first.Status != "submitted" {
			continue
		}
		for _, index := range group {
			item := plan.Items[index]
			actual, ok := allowed[item.Target.Key]
			if !ok || actual.UID != item.Target.UID || actual.Generation != item.AppliedGeneration || actual.Image != item.Image.Pinned {
				return infrastructureOperation(operation), nil
			}
		}
		workload, err := infrastructureexec.ReadWorkload(ctx, runner, access.Context, first.Target.Kind, first.Target.Namespace, first.Target.Workload)
		if err != nil {
			return InfrastructureOperation{}, err
		}
		expected := []infrastructureexec.ExpectedImage{}
		for _, index := range group {
			item := plan.Items[index]
			expected = append(expected, infrastructureexec.ExpectedImage{Container: item.Target.Container, Image: item.Image.Pinned, Digests: item.Image.Digests})
		}
		if err := infrastructureexec.Verify(ctx, runner, access.Context, workload, expected); err != nil {
			return infrastructureOperation(operation), nil
		}
		for _, index := range group {
			plan.Items[index].Status = "succeeded"
		}
		plan.Steps = append(plan.Steps, ImageReleaseStep{Kind: "verify", TargetKey: first.Target.Key, Status: "succeeded"})
	}
	status, errorText := "succeeded", ""
	for _, item := range plan.Items {
		if item.Status != "succeeded" {
			status, errorText = "failed", "some selected containers were not executed; create a new preview for the remaining roles"
			break
		}
	}
	if err := s.persistImageOperation(operation, status, errorText, plan); err != nil {
		return InfrastructureOperation{}, err
	}
	details, _ := json.Marshal(plan)
	operation.Status, operation.Error, operation.Details = status, errorText, details
	return infrastructureOperation(operation), nil
}

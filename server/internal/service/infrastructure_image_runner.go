package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/internal/infrastructureexec"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func imageGroups(plan ImageReleasePlan) [][]int {
	groups := [][]int{}
	positions := map[string]int{}
	for index, item := range plan.Items {
		key := item.Target.Kind + "/" + item.Target.Namespace + "/" + item.Target.Workload
		position, exists := positions[key]
		if !exists {
			position = len(groups)
			positions[key] = position
			groups = append(groups, []int{})
		}
		groups[position] = append(groups[position], index)
	}
	return groups
}

func (s *InfrastructureService) runImageOperation(operation db.InfrastructureOperation) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	var plan ImageReleasePlan
	if err := json.Unmarshal(operation.Details, &plan); err != nil || len(plan.Items) == 0 {
		_ = s.persistImageOperation(operation, "failed", "invalid stored image preview", plan)
		return
	}
	stepIndex := len(plan.Steps)
	plan.Steps = append(plan.Steps, ImageReleaseStep{Kind: "check", Status: "running"})
	finish := func(status string, err error) {
		if stepIndex < len(plan.Steps) {
			plan.Steps[stepIndex].Status, plan.Steps[stepIndex].Error = "failed", imageErrorDetail(err)
		}
		for index := range plan.Steps {
			if plan.Steps[index].Status == "pending" {
				plan.Steps[index].Status = "skipped"
			}
		}
		for index := range plan.Items {
			item := &plan.Items[index]
			switch item.Status {
			case "resolved":
				item.Status = "skipped"
			case "submitting":
				item.Status = status
			case "submitted":
				item.Status = "verification_required"
			}
		}
		_ = s.persistImageOperation(operation, status, imageErrorDetail(err), plan)
	}
	if err := s.persistImageOperation(operation, "running", "", plan); err != nil {
		return
	}
	row, err := s.Queries.GetInfrastructureConnection(ctx, db.GetInfrastructureConnectionParams{ID: operation.ConnectionID, WorkspaceID: operation.WorkspaceID})
	if err != nil {
		finish("failed", err)
		return
	}
	if row.UpdatedAt.Time.UTC().Format(time.RFC3339Nano) != plan.ConfigRevision {
		finish("conflict", errors.New("Manager or execution settings changed; create a new preview"))
		return
	}
	resources, err := s.imageResources(ctx, row, operation.ClusterID, operation.ServiceID)
	if err != nil {
		finish("failed", err)
		return
	}
	actual := map[string]infrastructureexec.Target{}
	for _, target := range resources.Targets {
		actual[target.Key] = target
	}
	for _, item := range plan.Items {
		target, exists := actual[item.Target.Key]
		if !exists || target.UID != item.Target.UID || target.Generation != item.Target.Generation || target.Image != item.Target.Image {
			finish("conflict", errors.New("a selected role deployment or image changed since preview; read and confirm again"))
			return
		}
	}
	access, configured, err := s.executionAccess(row, operation.ClusterID)
	if err != nil || !configured {
		finish("failed", errors.New("cluster execution access is no longer configured"))
		return
	}
	runner, err := s.dialExecution(ctx, access)
	if err != nil {
		finish("failed", err)
		return
	}
	defer runner.Close()
	groups := imageGroups(plan)
	plan.Steps[stepIndex].Status = "succeeded"
	for _, group := range groups {
		for _, kind := range []string{"update", "rollout", "verify"} {
			plan.Steps = append(plan.Steps, ImageReleaseStep{Kind: kind, TargetKey: plan.Items[group[0]].Target.Key, Status: "pending"})
		}
	}
	if err := s.persistImageOperation(operation, "running", "", plan); err != nil {
		return
	}
	for _, group := range groups {
		first := plan.Items[group[0]].Target
		stepIndex++
		plan.Steps[stepIndex].Status = "running"
		if err := s.persistImageOperation(operation, "running", "", plan); err != nil {
			return
		}
		workload, err := infrastructureexec.ReadWorkload(ctx, runner, access.Context, first.Kind, first.Namespace, first.Workload)
		if err != nil {
			finish("failed", err)
			return
		}
		patchItems := []infrastructureexec.PatchItem{}
		for _, index := range group {
			item := &plan.Items[index]
			patchItems = append(patchItems, infrastructureexec.PatchItem{Target: item.Target, Image: item.Image.Pinned})
		}
		restartID := ""
		if operation.Kind == "restart_pods" {
			restartID = uuidString(operation.ID)
		}
		patch, err := infrastructureexec.ImagePatch(workload, patchItems, restartID)
		if err != nil {
			finish("conflict", err)
			return
		}
		for _, index := range group {
			plan.Items[index].Status = "submitting"
		}
		// Persist the submission intent before sending a Kubernetes write.
		if err := s.persistImageOperation(operation, "running", "", plan); err != nil {
			return
		}
		resource, _ := infrastructureexec.Resource(first.Kind)
		output, err := runner.Run(ctx, infrastructureexec.Kube(access.Context, first.Namespace, "patch", resource, first.Workload, "--type=json", "--patch", string(patch), "-o", "json")...)
		if err != nil {
			status := "unknown"
			if infrastructureexec.DefiniteFailure(err) {
				status = "conflict"
			}
			finish(status, fmt.Errorf("Kubernetes patch was not confirmed; inspect the role before retrying: %w", err))
			return
		}
		var applied infrastructureexec.Workload
		if err := json.Unmarshal(output, &applied); err != nil || applied.Metadata.UID != first.UID || applied.Metadata.Generation <= 0 {
			finish("unknown", errors.New("Kubernetes patch response was not a matching workload; inspect the role before retrying"))
			return
		}
		for _, index := range group {
			plan.Items[index].Status, plan.Items[index].AppliedGeneration = "submitted", applied.Metadata.Generation
		}
		plan.Steps[stepIndex].Status = "succeeded"
		stepIndex++
		plan.Steps[stepIndex].Status = "running"
		if err := s.persistImageOperation(operation, "running", "", plan); err != nil {
			return
		}
		// rollout has its own deadline; a short API request timeout must not abort its watch.
		_, err = runner.Run(ctx, infrastructureexec.Kubectl(access.Context, "--namespace", first.Namespace, "rollout", "status", resource+"/"+first.Workload, "--timeout=300s")...)
		if err != nil {
			finish("verification_required", fmt.Errorf("role deployment was updated but rollout is not verified: %w", err))
			return
		}
		plan.Steps[stepIndex].Status = "succeeded"
		stepIndex++
		plan.Steps[stepIndex].Status = "running"
		if err := s.persistImageOperation(operation, "running", "", plan); err != nil {
			return
		}
		current, err := infrastructureexec.ReadWorkload(ctx, runner, access.Context, first.Kind, first.Namespace, first.Workload)
		if err != nil {
			finish("verification_required", err)
			return
		}
		if current.Metadata.UID != first.UID || current.Metadata.Generation != plan.Items[group[0]].AppliedGeneration {
			finish("verification_required", errors.New("role deployment changed during rollout; inspect its current version"))
			return
		}
		expected := []infrastructureexec.ExpectedImage{}
		for _, index := range group {
			item := plan.Items[index]
			expected = append(expected, infrastructureexec.ExpectedImage{Container: item.Target.Container, Image: item.Image.Pinned, Digests: item.Image.Digests})
		}
		if err := infrastructureexec.Verify(ctx, runner, access.Context, current, expected); err != nil {
			finish("verification_required", err)
			return
		}
		for _, index := range group {
			plan.Items[index].Status = "succeeded"
		}
		plan.Steps[stepIndex].Status = "succeeded"
		if err := s.persistImageOperation(operation, "running", "", plan); err != nil {
			return
		}
	}
	_ = s.persistImageOperation(operation, "succeeded", "", plan)
}

func imageErrorDetail(err error) string {
	message := strings.ToValidUTF8(err.Error(), "?")
	if len(message) > 8192 {
		message = message[:8192]
	}
	return message
}

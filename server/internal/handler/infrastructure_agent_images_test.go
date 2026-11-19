package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestAgentServiceImagesAuthorizationAuditAndTaskBoundSubmission(t *testing.T) {
	connection, node := imageFixture(t)
	runtime := dbfx.Runtime(t, "Image agent runtime")
	agent := dbfx.Agent(t, "Image publisher", runtime)
	issue := dbfx.Issue(t, "Publish requested role images")
	task := dbfx.Task(t, agent, testutil.Cols{"issue_id": issue, "runtime_id": runtime, "status": "running", "originator_user_id": testUserID, "accountable_user_id": testUserID})
	otherTask := dbfx.Task(t, agent, testutil.Cols{"issue_id": issue, "runtime_id": runtime, "status": "running", "originator_user_id": testUserID, "accountable_user_id": testUserID})
	member := dbfx.User(t, "Image task member", "image-task-member@test.local")
	dbfx.Member(t, testWorkspaceID, member, "member")
	input := map[string]any{"connection_id": connection, "cluster_id": 7, "service_id": 8, "action": "publish_images", "reason": "publish requested role fixes", "items": []map[string]string{{"key": "deployment/qa/worker-a/worker", "source": "harbor.example/team/worker:custom_fix"}, {"key": "deployment/qa/coordinator-a/coordinator", "source": "harbor.example/team/coordinator:different_fix"}}}
	request := func(tool, taskID string) *http.Request {
		r := infrastructureRequest(t, tool, input)
		r.Header.Set("X-Actor-Source", "task_token")
		r.Header.Set("X-Agent-ID", agent)
		r.Header.Set("X-Task-ID", taskID)
		return r
	}
	var resources service.ImageResources
	testutil.Call(t, testHandler.InvokeInfrastructureTool, request("list_service_images", task)).Want(200).JSON(&resources)
	if !resources.Configured || len(resources.Targets) != 4 {
		t.Fatal("agent cannot discover the service's actual role images")
	}
	delete(input, "reason")
	testutil.Call(t, testHandler.InvokeInfrastructureTool, request("preview_service_images", task)).Want(400)
	input["reason"] = "publish requested role fixes"
	var operation service.InfrastructureOperation
	testutil.Call(t, testHandler.InvokeInfrastructureTool, request("preview_service_images", task)).Want(200).JSON(&operation)
	operation = awaitImageOperation(t, connection, operation.ID)
	var plan service.ImageReleasePlan
	if err := json.Unmarshal(operation.Details, &plan); err != nil {
		t.Fatal(err)
	}
	if operation.Status != "ready" || operation.ActorType != "agent" || operation.ActorID != agent || plan.TaskID != task || plan.OriginatorUserID != testUserID || plan.Items[0].Image.Digest == plan.Items[1].Image.Digest {
		t.Fatalf("incorrect task image preview: %+v", operation)
	}
	input["operation_id"] = operation.ID
	testutil.Call(t, testHandler.InvokeInfrastructureTool, request("start_service_images", otherTask)).Want(409)
	dbfx.Exec(t, `UPDATE agent_task_queue SET originator_user_id=$1,accountable_user_id=$1 WHERE id=$2`, member, task)
	testutil.Call(t, testHandler.InvokeInfrastructureTool, request("start_service_images", task)).Want(403)
	testutil.Call(t, testHandler.InvokeInfrastructureTool, request("preview_service_images", task)).Want(403)
	dbfx.Exec(t, `UPDATE agent_task_queue SET originator_user_id=NULL WHERE id=$1`, task)
	testutil.Call(t, testHandler.InvokeInfrastructureTool, request("start_service_images", task)).Want(403)
	dbfx.Exec(t, `UPDATE agent_task_queue SET originator_user_id=$1,accountable_user_id=$1 WHERE id=$2`, testUserID, task)
	// MCP calls use the daemon/task binding gate and the same scoped dispatcher.
	r := newOwnerDaemonRequest(http.MethodPost, "/api/daemon/tasks/"+task+"/infrastructure/tools/start_service_images", input)
	route := chi.NewRouteContext()
	route.URLParams.Add("id", task)
	route.URLParams.Add("tool", "start_service_images")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, route))
	testutil.Call(t, testHandler.InvokeAgentInfrastructureTool, r).Want(200)
	operation = awaitImageOperation(t, connection, operation.ID)
	if operation.Status != "succeeded" {
		t.Fatalf("agent publication failed: %s %s", operation.Status, operation.Error)
	}
	testutil.Call(t, testHandler.InvokeInfrastructureTool, request("start_service_images", task)).Want(409)
	dbfx.Exec(t, `UPDATE agent_task_queue SET status='completed' WHERE id=$1`, task)
	testutil.Call(t, testHandler.InvokeInfrastructureTool, request("preview_service_images", task)).Want(409)
	node.mu.Lock()
	defer node.mu.Unlock()
	if len(node.patched) != 2 {
		t.Fatalf("unexpected writes: %v", node.patched)
	}
}

package handler

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/integrations/transwarp"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util/secretbox"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func infrastructureFixture(t *testing.T, workspace, name string, box *secretbox.Box) (string, *atomic.Int32) {
	t.Helper()
	restarts := &atomic.Int32{}
	var metaContent atomic.Value
	metaContent.Store("name: " + name)
	manager := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/users/login":
			http.SetCookie(w, &http.Cookie{Name: "session", Value: name, Path: "/"})
			json.NewEncoder(w).Encode(map[string]any{})
		case "/api/users/profile":
			cookie, err := r.Cookie("session")
			if err != nil || cookie.Value != name {
				w.WriteHeader(401)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"userName": "ops"})
		case "/api/clusters":
			json.NewEncoder(w).Encode([]map[string]any{{"id": 7, "name": "Cluster"}})
		case "/api/nodes":
			json.NewEncoder(w).Encode([]map[string]any{{"id": 1, "hostName": name, "clusterId": 7}})
		case "/api/services/8":
			json.NewEncoder(w).Encode(map[string]any{"id": 8, "type": "APP", "version": "v1"})
		case "/api/toolbox/metaEditor/tree/service":
			json.NewEncoder(w).Encode([]map[string]any{{"key": "APP/v1/metainfo.yaml", "title": "metainfo.yaml", "type": "INFO_YAML", "isLeaf": true}})
		case "/api/toolbox/metaEditor/file/text":
			if r.Method == http.MethodPost {
				var input map[string]string
				if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input["category"] != "SERVICE" {
					t.Error("metainfo save lost service category")
				}
				restarts.Add(1)
				if name == "manager-meta-fail" {
					w.WriteHeader(500)
					return
				}
				metaContent.Store(input["content"])
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"text": true, "content": metaContent.Load()})
		case "/api/serviceRoles/9":
			json.NewEncoder(w).Encode(map[string]any{"id": 9, "name": "Worker", "serviceId": 8, "installed": true, "frontendStatus": "RUNNING", "health": "HEALTHY", "node": map[string]any{"id": 1, "hostName": name, "clusterId": 7}})
		case "/api/serviceRoles/operations/canStop":
			json.NewEncoder(w).Encode(map[string]any{"stoppable": true})
		case "/api/serviceRoles/operations/restart":
			restarts.Add(1)
			json.NewEncoder(w).Encode(map[string]any{"id": 29})
		case "/api/operations/jobs/29":
			json.NewEncoder(w).Encode(map[string]any{"id": 29, "status": "SUCCESSFUL"})
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(manager.Close)
	sealed, err := box.Seal([]byte("test-password"))
	if err != nil {
		t.Fatal(err)
	}
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: manager.Certificate().Raw})
	id := dbfx.Insert(t, "infrastructure_connection", testutil.Cols{"workspace_id": workspace, "name": name, "base_url": manager.URL, "username": "ops", "password_ciphertext": sealed, "ca_pem": string(cert), "created_by": testUserID})
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM activity_log WHERE workspace_id = $1 AND details->>'connection_id' = $2`, workspace, id)
		testHandler.Queries.DeleteInfrastructureOperations(context.Background(), db.DeleteInfrastructureOperationsParams{ConnectionID: parseUUID(id), WorkspaceID: parseUUID(workspace)})
	})
	return id, restarts
}

func TestInfrastructureServiceMetainfoPermissionsAndWorkspaceScope(t *testing.T) {
	box, err := secretbox.New(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	previous := testHandler.Infrastructure
	testHandler.Infrastructure = &service.InfrastructureService{Queries: testHandler.Queries, TxStarter: testPool, Secrets: box}
	t.Cleanup(func() { testHandler.Infrastructure = previous })
	a, writes := infrastructureFixture(t, testWorkspaceID, "manager-meta-a", box)
	b, _ := infrastructureFixture(t, testWorkspaceID, "manager-meta-b", box)
	foreignWS := dbfx.Workspace(t, "Foreign metainfo", "foreign-metainfo")
	foreign, _ := infrastructureFixture(t, foreignWS, "foreign-metainfo", box)
	member := dbfx.User(t, "Metadata reader", "metainfo-reader@test.local")
	dbfx.Member(t, testWorkspaceID, member, "member")
	router := chi.NewRouter()
	router.Route("/api/workspaces/{id}", func(r chi.Router) {
		r.With(middleware.RequireWorkspaceMemberFromURL(testHandler.Queries, "id")).Get("/infrastructure/connections/{connectionId}/services/{serviceId}/metainfo", testHandler.InfrastructureServiceMeta)
		r.With(middleware.RequireWorkspaceRoleFromURL(testHandler.Queries, "id", "owner", "admin"), RequireHumanActor).Put("/infrastructure/connections/{connectionId}/services/{serviceId}/metainfo", testHandler.InfrastructureServiceMeta)
	})
	path := func(id string) string {
		return "/api/workspaces/" + testWorkspaceID + "/infrastructure/connections/" + id + "/services/8/metainfo?key=APP%2Fv1%2Fmetainfo.yaml"
	}
	for _, item := range []struct{ id, name string }{{a, "manager-meta-a"}, {b, "manager-meta-b"}} {
		request := newRequest(http.MethodGet, path(item.id), nil)
		request.Header.Set("X-User-ID", member)
		var file transwarp.MetaFile
		testutil.Call(t, router.ServeHTTP, request).Want(200).JSON(&file)
		if file.Content != "name: "+item.name {
			t.Fatalf("Manager isolation failed: %+v", file)
		}
	}
	input := map[string]string{"content": "name: changed"}
	request := newRequest(http.MethodPut, path(a), input)
	request.Header.Set("X-User-ID", member)
	testutil.Call(t, router.ServeHTTP, request).Want(403)
	request = newRequest(http.MethodPut, path(a), input)
	request.Header.Set("X-Actor-Source", "task_token")
	testutil.Call(t, router.ServeHTTP, request).Want(403)
	if writes.Load() != 0 {
		t.Fatal("unauthorized edit reached Manager")
	}
	testutil.Call(t, router.ServeHTTP, newRequest(http.MethodGet, path(foreign), nil)).Want(404)
	testutil.Call(t, router.ServeHTTP, newRequest(http.MethodPut, path(foreign), input)).Want(404)
	testutil.Call(t, router.ServeHTTP, newRequest(http.MethodPut, path(a), map[string]any{})).Want(400)
	var saved service.MetainfoSaveResult
	testutil.Call(t, router.ServeHTTP, newRequest(http.MethodPut, path(a), input)).Want(200).JSON(&saved)
	if !saved.Saved || saved.AuditID == "" || writes.Load() != 1 {
		t.Fatal("admin edit did not save exactly once")
	}
	testutil.Call(t, http.HandlerFunc(testHandler.InvokeInfrastructureTool), infrastructureRequest(t, "inspect_cluster", map[string]any{"connection_id": a, "cluster_id": 7})).Want(404)
}

func infrastructureRequest(t *testing.T, tool string, input any) *http.Request {
	t.Helper()
	req := newRequest(http.MethodPost, "/api/workspaces/"+testWorkspaceID+"/infrastructure/tools/"+tool, input)
	route := chi.NewRouteContext()
	route.URLParams.Add("id", testWorkspaceID)
	route.URLParams.Add("tool", tool)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, route))
}

func TestInfrastructureAgentMetainfoReadEditPermissionsAndAudit(t *testing.T) {
	box, err := secretbox.New(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	previous := testHandler.Infrastructure
	testHandler.Infrastructure = &service.InfrastructureService{Queries: testHandler.Queries, TxStarter: testPool, Secrets: box}
	t.Cleanup(func() { testHandler.Infrastructure = previous })
	connection, writes := infrastructureFixture(t, testWorkspaceID, "manager-agent-meta", box)
	otherWS := dbfx.Workspace(t, "Other agent metadata", "other-agent-meta")
	foreign, _ := infrastructureFixture(t, otherWS, "foreign-agent-meta", box)
	member := dbfx.User(t, "Metadata member", "meta-member@test.local")
	dbfx.Member(t, testWorkspaceID, member, "member")
	runtime := dbfx.Runtime(t, "Metadata runtime")
	agentID := dbfx.Agent(t, "Metadata agent", runtime)
	issue := dbfx.Issue(t, "Metadata task")
	taskID := dbfx.Task(t, agentID, testutil.Cols{"issue_id": issue, "runtime_id": runtime, "status": "running", "originator_user_id": testUserID, "accountable_user_id": testUserID})
	cliRequest := func(tool string, input any) *http.Request {
		request := infrastructureRequest(t, tool, input)
		request.Header.Set("X-Actor-Source", "task_token")
		request.Header.Set("X-Agent-ID", agentID)
		request.Header.Set("X-Task-ID", taskID)
		return request
	}
	input := map[string]any{"connection_id": connection, "service_id": 8, "key": "APP/v1/metainfo.yaml", "content": "name: changed-by-agent\n", "reason": "update metadata"}
	var files []transwarp.MetaEntry
	testutil.Call(t, testHandler.InvokeInfrastructureTool, cliRequest("list_metainfo_files", input)).Want(200).JSON(&files)
	if len(files) != 1 || files[0].Key != input["key"] {
		t.Fatalf("missing file identity: %+v", files)
	}
	var file transwarp.MetaFile
	testutil.Call(t, testHandler.InvokeInfrastructureTool, cliRequest("read_metainfo_file", input)).Want(200).JSON(&file)
	if file.Content != "name: manager-agent-meta" {
		t.Fatalf("wrong Manager content: %+v", file)
	}
	var saved service.MetainfoSaveResult
	testutil.Call(t, testHandler.InvokeInfrastructureTool, cliRequest("save_metainfo_file", input)).Want(200).JSON(&saved)
	if !saved.Saved || saved.AuditID == "" || writes.Load() != 1 {
		t.Fatalf("save=%+v writes=%d", saved, writes.Load())
	}
	testutil.Call(t, testHandler.InvokeInfrastructureTool, cliRequest("read_metainfo_file", input)).Want(200).JSON(&file)
	if file.Content != input["content"] {
		t.Fatal("saved content did not round trip")
	}
	var action, actorType, recordedActor, details string
	dbfx.QueryRow(t, `SELECT action, actor_type, actor_id::text, details::text FROM activity_log WHERE id = $1`, saved.AuditID).Scan(&action, &actorType, &recordedActor, &details)
	if action != "infrastructure_metainfo_save_requested" || actorType != "agent" || recordedActor != agentID || !strings.Contains(details, taskID) || strings.Contains(details, "changed-by-agent") {
		t.Fatalf("wrong audit attribution or leaked file body: action=%s actor=%s/%s details=%s", action, actorType, recordedActor, details)
	}
	var outcomes int
	dbfx.QueryRow(t, `SELECT count(*) FROM activity_log WHERE details->>'audit_id' = $1 AND action = 'infrastructure_metainfo_saved'`, saved.AuditID).Scan(&outcomes)
	if outcomes != 1 {
		t.Fatal("save outcome not recorded")
	}

	// The daemon owner is an admin; an ordinary originator must not inherit that authority.
	dbfx.Exec(t, `UPDATE agent_task_queue SET originator_user_id = $1, accountable_user_id = $1 WHERE id = $2`, member, taskID)
	testutil.Call(t, testHandler.InvokeInfrastructureTool, cliRequest("save_metainfo_file", input)).Want(403)
	testutil.Call(t, testHandler.InvokeInfrastructureTool, cliRequest("read_metainfo_file", input)).Want(200)
	dbfx.Exec(t, `UPDATE agent_task_queue SET originator_user_id = NULL WHERE id = $1`, taskID)
	testutil.Call(t, testHandler.InvokeInfrastructureTool, cliRequest("save_metainfo_file", input)).Want(403)
	dbfx.Exec(t, `UPDATE agent_task_queue SET originator_user_id = $1, accountable_user_id = $1 WHERE id = $2`, testUserID, taskID)
	input["connection_id"] = foreign
	testutil.Call(t, testHandler.InvokeInfrastructureTool, cliRequest("read_metainfo_file", input)).Want(404)
	testutil.Call(t, testHandler.InvokeInfrastructureTool, cliRequest("save_metainfo_file", input)).Want(404)
	input["connection_id"] = connection
	input["key"] = "OTHER/v1/metainfo.yaml"
	testutil.Call(t, testHandler.InvokeInfrastructureTool, cliRequest("save_metainfo_file", input)).Want(400)
	input["key"] = "APP/v1/metainfo.yaml"
	delete(input, "content")
	testutil.Call(t, testHandler.InvokeInfrastructureTool, cliRequest("save_metainfo_file", input)).Want(400)
	input["content"] = "name: changed-again"
	delete(input, "reason")
	testutil.Call(t, testHandler.InvokeInfrastructureTool, cliRequest("save_metainfo_file", input)).Want(400)
	input["reason"] = "update metadata"
	if writes.Load() != 1 {
		t.Fatal("a denied edit reached Manager")
	}

	// Native MCP calls pass through the daemon's task/runtime binding gate.
	request := newOwnerDaemonRequest(http.MethodPost, "/api/daemon/tasks/"+taskID+"/infrastructure/tools/save_metainfo_file", input)
	route := chi.NewRouteContext()
	route.URLParams.Add("id", taskID)
	route.URLParams.Add("tool", "save_metainfo_file")
	request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, route))
	testutil.Call(t, testHandler.InvokeAgentInfrastructureTool, request).Want(200)
	if writes.Load() != 2 {
		t.Fatal("native MCP save did not reach Manager")
	}
	dbfx.Exec(t, `UPDATE agent_task_queue SET status = 'completed' WHERE id = $1`, taskID)
	testutil.Call(t, testHandler.InvokeInfrastructureTool, cliRequest("save_metainfo_file", input)).Want(409)
}

func TestInfrastructureMetainfoUncertainSaveIsNotRetried(t *testing.T) {
	box, err := secretbox.New(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	previous := testHandler.Infrastructure
	testHandler.Infrastructure = &service.InfrastructureService{Queries: testHandler.Queries, TxStarter: testPool, Secrets: box}
	t.Cleanup(func() { testHandler.Infrastructure = previous })
	connection, writes := infrastructureFixture(t, testWorkspaceID, "manager-meta-fail", box)
	testutil.Call(t, testHandler.InvokeInfrastructureTool, infrastructureRequest(t, "save_metainfo_file", map[string]any{"connection_id": connection, "service_id": 8, "key": "APP/v1/metainfo.yaml", "content": "name: uncertain", "reason": "update metadata"})).Want(400)
	if writes.Load() != 1 {
		t.Fatal("uncertain save was retried")
	}
	var outcomes int
	dbfx.QueryRow(t, `SELECT count(*) FROM activity_log WHERE details->>'connection_id' = $1 AND action = 'infrastructure_metainfo_save_unconfirmed'`, connection).Scan(&outcomes)
	if outcomes != 1 {
		t.Fatal("uncertain save was not recorded")
	}
}

func TestInfrastructureWorkspaceIsolationAndAuditedRestart(t *testing.T) {
	box, err := secretbox.New(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	previous := testHandler.Infrastructure
	testHandler.Infrastructure = &service.InfrastructureService{Queries: testHandler.Queries, TxStarter: testPool, Secrets: box}
	t.Cleanup(func() { testHandler.Infrastructure = previous })
	a, calls := infrastructureFixture(t, testWorkspaceID, "manager-a", box)
	b, _ := infrastructureFixture(t, testWorkspaceID, "manager-b", box)
	otherWorkspace := dbfx.Workspace(t, "Other infrastructure workspace", "infra-other")
	foreign, _ := infrastructureFixture(t, otherWorkspace, "foreign-manager", box)
	for _, item := range []struct{ id, name string }{{a, "manager-a"}, {b, "manager-b"}, {a, "manager-a"}} {
		var nodes []transwarp.Node
		testutil.Call(t, http.HandlerFunc(testHandler.InvokeInfrastructureTool), infrastructureRequest(t, "list_nodes", map[string]any{"connection_id": item.id, "cluster_id": 7})).Want(200).JSON(&nodes)
		if len(nodes) != 1 || nodes[0].Name != item.name {
			t.Fatalf("mixed Manager identities: %+v", nodes)
		}
	}
	testutil.Call(t, http.HandlerFunc(testHandler.InvokeInfrastructureTool), infrastructureRequest(t, "list_nodes", map[string]any{"connection_id": foreign, "cluster_id": 7})).Want(404)
	input := map[string]any{"connection_id": a, "cluster_id": 7, "service_id": 8, "role_id": 9, "reason": "recover unhealthy worker"}
	testutil.Call(t, http.HandlerFunc(testHandler.InvokeInfrastructureTool), infrastructureRequest(t, "restart_role", input)).Want(400)
	if calls.Load() != 0 {
		t.Fatal("restart without a grant reached Manager")
	}
	_, err = testHandler.Infrastructure.SetGrants(context.Background(), parseUUID(testWorkspaceID), parseUUID(a), []service.InfrastructureGrant{{ClusterID: 7, ServiceID: 8, RoleID: 9}})
	if err != nil {
		t.Fatal(err)
	}
	input["cluster_id"] = 70
	testutil.Call(t, http.HandlerFunc(testHandler.InvokeInfrastructureTool), infrastructureRequest(t, "restart_role", input)).Want(400)
	if calls.Load() != 0 {
		t.Fatal("grant crossed cluster boundary")
	}
	input["cluster_id"] = 7
	var operation service.InfrastructureOperation
	testutil.Call(t, http.HandlerFunc(testHandler.InvokeInfrastructureTool), infrastructureRequest(t, "restart_role", input)).Want(200).JSON(&operation)
	if operation.Status != "submitted" || operation.JobID == nil || calls.Load() != 1 {
		t.Fatalf("wrong submission: %+v calls=%d", operation, calls.Load())
	}
	var verified service.InfrastructureOperation
	testutil.Call(t, http.HandlerFunc(testHandler.InvokeInfrastructureTool), infrastructureRequest(t, "get_operation", map[string]any{"connection_id": a, "operation_id": operation.ID})).Want(200).JSON(&verified)
	if verified.Status != "succeeded" {
		t.Fatalf("job and health were not verified: %+v", verified)
	}
	rows, err := testHandler.Infrastructure.List(context.Background(), parseUUID(testWorkspaceID))
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(rows)
	if strings.Contains(string(encoded), "test-password") || strings.Contains(string(encoded), "password_ciphertext") {
		t.Fatal("credentials leaked into connection metadata")
	}
	if err := testHandler.Infrastructure.Delete(context.Background(), parseUUID(testWorkspaceID), parseUUID(a)); err != nil {
		t.Fatal(err)
	}
	operations, err := testHandler.Queries.ListInfrastructureOperations(context.Background(), db.ListInfrastructureOperationsParams{WorkspaceID: parseUUID(testWorkspaceID), ConnectionID: parseUUID(a)})
	if err != nil || len(operations) != 0 {
		t.Fatalf("dependent operation cleanup failed: %v", err)
	}
}

func TestInfrastructureCertificateBypassSurvivesConnectionSaveAndReload(t *testing.T) {
	box, err := secretbox.New(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	id, _ := infrastructureFixture(t, testWorkspaceID, "manager-tls", box)
	row, err := testHandler.Queries.GetInfrastructureConnection(context.Background(), db.GetInfrastructureConnectionParams{ID: parseUUID(id), WorkspaceID: parseUUID(testWorkspaceID)})
	if err != nil {
		t.Fatal(err)
	}
	svc := &service.InfrastructureService{Queries: testHandler.Queries, TxStarter: testPool, Secrets: box}
	input := service.InfrastructureConnectionInput{Name: row.Name, URL: row.BaseUrl, Username: row.Username, Password: "test-password", SkipTLSVerify: true}
	saved, err := svc.Save(context.Background(), parseUUID(testWorkspaceID), parseUUID(id), parseUUID(testUserID), input)
	if err != nil || !saved.SkipTLSVerify {
		t.Fatalf("could not save bypass: %+v err=%v", saved, err)
	}
	// A fresh service instance forces credentials and TLS settings to load from the database.
	reloaded := &service.InfrastructureService{Queries: testHandler.Queries, TxStarter: testPool, Secrets: box}
	if _, err := reloaded.Execute(context.Background(), parseUUID(testWorkspaceID), parseUUID(id), parseUUID(testUserID), "user", "list_clusters", service.InfrastructureInput{}); err != nil {
		t.Fatalf("stored bypass was not used: %v", err)
	}
	input.SkipTLSVerify = false
	input.CAPEM = row.CaPem
	verified, err := svc.Save(context.Background(), parseUUID(testWorkspaceID), parseUUID(id), parseUUID(testUserID), input)
	if err != nil || verified.SkipTLSVerify {
		t.Fatalf("could not restore CA verification: %+v err=%v", verified, err)
	}
}

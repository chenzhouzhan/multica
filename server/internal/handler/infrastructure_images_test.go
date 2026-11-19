package handler

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/infrastructureexec"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util/secretbox"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"golang.org/x/crypto/ssh"
)

type imageTestNode struct {
	mu             sync.Mutex
	workloads      map[string]map[string]any
	patched        []string
	failRollout    string
	uncertainPatch bool
}

func newImageTestNode() *imageTestNode {
	node := &imageTestNode{workloads: map[string]map[string]any{}}
	for _, name := range []string{"worker-a", "coordinator-a"} {
		role := strings.TrimSuffix(name, "-a")
		node.workloads[name] = map[string]any{"kind": "Deployment", "metadata": map[string]any{"name": name, "namespace": "qa", "uid": "uid-" + name, "generation": float64(7)}, "spec": map[string]any{"replicas": float64(1), "selector": map[string]any{"matchLabels": map[string]string{"app": "shared"}}, "template": map[string]any{"metadata": map[string]any{}, "spec": map[string]any{"containers": []any{map[string]any{"name": role, "image": "manager.example:5000/team/" + role + ":old-nonstandard"}, map[string]any{"name": "metrics", "image": "manager.example:5000/team/metrics:keep"}}}}}, "status": map[string]any{"observedGeneration": float64(7)}}
	}
	return node
}

func imageTestDigest(source string) string {
	value := sha256.Sum256([]byte(source))
	return "sha256:" + hex.EncodeToString(value[:])
}
func imageTestArg(args []string, key string) string {
	for index, arg := range args {
		if arg == key && index+1 < len(args) {
			return args[index+1]
		}
	}
	return ""
}
func (n *imageTestNode) Close() error { return nil }
func (n *imageTestNode) Run(_ context.Context, args ...string) ([]byte, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if args[0] == "docker" {
		if args[1] == "version" {
			return []byte("28.0.0"), nil
		}
		if args[1] == "pull" {
			return []byte("Digest: " + imageTestDigest(args[2]) + "\n"), nil
		}
		pinned := args[len(args)-1]
		_, digest, _ := strings.Cut(pinned, "@")
		return json.Marshal(map[string]any{"Descriptor": map[string]string{"digest": digest}})
	}
	if len(args) > 1 && args[1] == "config" {
		return []byte(`{"contexts":[{"name":"test","context":{"cluster":"test"}}]}`), nil
	}
	for _, arg := range args {
		if arg == "--raw=/version" {
			return []byte(`{"gitVersion":"v1.30"}`), nil
		}
	}
	if name := imageTestArg(args, "status"); name != "" {
		workload := strings.TrimPrefix(name, "deployment/")
		if workload == n.failRollout {
			return nil, errors.New("rollout timed out: ImagePullBackOff")
		}
		return []byte("successfully rolled out"), nil
	}
	if resource := imageTestArg(args, "patch"); resource != "" {
		name := imageTestArg(args, resource)
		workload := n.workloads[name]
		if workload == nil {
			return nil, errors.New("foreign workload")
		}
		var patch []struct {
			Op, Path string
			Value    any
		}
		if err := json.Unmarshal([]byte(imageTestArg(args, "--patch")), &patch); err != nil {
			return nil, err
		}
		containers := workload["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["containers"].([]any)
		for _, op := range patch {
			if op.Op == "add" && op.Path == "/spec/template/metadata/annotations" {
				workload["spec"].(map[string]any)["template"].(map[string]any)["metadata"].(map[string]any)["annotations"] = op.Value
			}
			if op.Op == "replace" {
				var index int
				if _, err := fmt.Sscanf(op.Path, "/spec/template/spec/containers/%d/image", &index); err != nil {
					return nil, err
				}
				containers[index].(map[string]any)["image"] = op.Value
			}
		}
		n.patched = append(n.patched, name)
		metadata := workload["metadata"].(map[string]any)
		metadata["generation"] = metadata["generation"].(float64) + 1
		workload["status"].(map[string]any)["observedGeneration"] = metadata["generation"]
		if n.uncertainPatch {
			return nil, errors.New("SSH disconnected after patch submission")
		}
		return json.Marshal(workload)
	}
	resource := imageTestArg(args, "get")
	if resource == "deployment" {
		return json.Marshal(n.workloads[imageTestArg(args, resource)])
	}
	if resource == "replicasets" {
		items := []any{}
		for name := range n.workloads {
			items = append(items, map[string]any{"metadata": map[string]any{"uid": "rs-" + name, "ownerReferences": []any{map[string]any{"uid": "uid-" + name, "controller": true}}}})
		}
		return json.Marshal(map[string]any{"items": items})
	}
	if resource == "pods" {
		items := []any{}
		for name, workload := range n.workloads {
			containers := workload["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["containers"].([]any)
			statuses := []any{}
			for _, value := range containers {
				container := value.(map[string]any)
				image := container["image"].(string)
				_, digest, pinned := strings.Cut(image, "@")
				if !pinned {
					digest = imageTestDigest(image)
				}
				statuses = append(statuses, map[string]any{"name": container["name"], "ready": true, "imageID": "docker-pullable://registry.example/image@" + digest})
			}
			ready := "True"
			if name == n.failRollout {
				ready = "False"
			}
			items = append(items, map[string]any{"metadata": map[string]any{"name": name + "-pod", "ownerReferences": []any{map[string]any{"uid": "rs-" + name, "controller": true}}}, "spec": map[string]any{"containers": containers}, "status": map[string]any{"phase": "Running", "conditions": []any{map[string]string{"type": "Ready", "status": ready}}, "containerStatuses": statuses}})
		}
		return json.Marshal(map[string]any{"items": items})
	}
	return nil, fmt.Errorf("unexpected fake node command: %v", args)
}

func imageFixture(t *testing.T) (string, *imageTestNode) {
	t.Helper()
	box, err := secretbox.New(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	node := newImageTestNode()
	manager := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/users/login":
			json.NewEncoder(w).Encode(map[string]any{})
		case "/api/users/profile":
			json.NewEncoder(w).Encode(map[string]string{"userName": "ops"})
		case "/api/clusters":
			json.NewEncoder(w).Encode([]map[string]any{{"id": 7, "name": "cluster"}})
		case "/api/services/8":
			json.NewEncoder(w).Encode(map[string]any{"id": 8, "clusterId": 7, "installed": true})
		case "/api/serviceRoles":
			json.NewEncoder(w).Encode([]map[string]any{{"id": 9, "name": "Worker", "roleType": "WORKER", "serviceId": 8}, {"id": 10, "name": "Coordinator", "roleType": "COORDINATOR", "serviceId": 8}})
		case "/api/services/8/tosObjects/kinds":
			json.NewEncoder(w).Encode([]string{"DEPLOYMENT"})
		case "/api/services/8/tosObjects":
			json.NewEncoder(w).Encode([]map[string]string{{"name": "worker-a"}, {"name": "coordinator-a"}})
		case "/api/services/8/tosObjects/config":
			name := r.URL.Query().Get("name")
			json.NewEncoder(w).Encode("kind: Deployment\nmetadata:\n  name: " + name + "\n  namespace: qa\n  uid: uid-" + name + "\n")
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(manager.Close)
	sealed, _ := box.Seal([]byte("manager-password"))
	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: manager.Certificate().Raw})
	connection := dbfx.Insert(t, "infrastructure_connection", testutil.Cols{"workspace_id": testWorkspaceID, "name": "images", "base_url": manager.URL, "username": "ops", "password_ciphertext": sealed, "ca_pem": string(certificate), "created_by": testUserID})
	previous := testHandler.Infrastructure
	testHandler.Infrastructure = &service.InfrastructureService{Queries: testHandler.Queries, TxStarter: testPool, Secrets: box, ExecutionDial: func(context.Context, infrastructureexec.Access) (infrastructureexec.Runner, error) { return node, nil }}
	t.Cleanup(func() {
		testHandler.Infrastructure = previous
		testHandler.Queries.DeleteInfrastructureOperations(context.Background(), db.DeleteInfrastructureOperationsParams{WorkspaceID: parseUUID(testWorkspaceID), ConnectionID: parseUUID(connection)})
	})
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key, _ := ssh.NewPublicKey(public)
	_, err = testHandler.Infrastructure.SaveExecutionAccess(context.Background(), parseUUID(testWorkspaceID), parseUUID(connection), 7, service.ExecutionAccessInput{Access: infrastructureexec.Access{Host: "manager.test", Port: 22, Username: "deploy", HostKey: string(ssh.MarshalAuthorizedKey(key)), Context: "test", Password: "node-password"}, AuthType: "password"})
	if err != nil {
		t.Fatal(err)
	}
	return connection, node
}

func imageRequest(t *testing.T, connection, method string, input any, operation string) *http.Request {
	t.Helper()
	request := newRequest(method, "/api/workspaces/"+testWorkspaceID+"/infrastructure/images", input)
	route := chi.NewRouteContext()
	for key, value := range map[string]string{"id": testWorkspaceID, "connectionId": connection, "clusterId": "7", "serviceId": "8", "operationId": operation} {
		route.URLParams.Add(key, value)
	}
	return request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, route))
}

func awaitImageOperation(t *testing.T, connection, id string) service.InfrastructureOperation {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		operation, err := testHandler.Infrastructure.RefreshOperation(context.Background(), parseUUID(testWorkspaceID), parseUUID(connection), parseUUID(id))
		if err != nil {
			t.Fatal(err)
		}
		if operation.Status != "preparing" && operation.Status != "running" {
			return operation
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("image operation did not finish")
	return service.InfrastructureOperation{}
}

func previewImage(t *testing.T, connection string, keys ...string) service.InfrastructureOperation {
	t.Helper()
	items := []service.ImageSelection{}
	for _, key := range keys {
		items = append(items, service.ImageSelection{Key: "deployment/qa/" + key + "-a/" + key, Source: "harbor.example/team/" + key + ":Li-debug_42"})
	}
	var operation service.InfrastructureOperation
	testutil.Call(t, testHandler.PreviewInfrastructureImages, imageRequest(t, connection, http.MethodPost, service.ImagePreviewInput{Action: "publish_images", Items: items, Reason: "verify selected roles"}, "")).Want(202).JSON(&operation)
	return awaitImageOperation(t, connection, operation.ID)
}

func TestInfrastructureImagesPermissionsScopeAndCredentialProjection(t *testing.T) {
	connection, node := imageFixture(t)
	member := dbfx.User(t, "Image member", "images-member@test.local")
	dbfx.Member(t, testWorkspaceID, member, "member")
	request := imageRequest(t, connection, http.MethodPost, service.ImagePreviewInput{}, "")
	request.Header.Set("X-User-ID", member)
	testutil.Call(t, testHandler.PreviewInfrastructureImages, request).Want(403)
	request = imageRequest(t, connection, http.MethodPost, service.ImagePreviewInput{}, "")
	request.Header.Set("X-Actor-Source", "task_token")
	testutil.Call(t, testHandler.PreviewInfrastructureImages, request).Want(403)
	var resources service.ImageResources
	testutil.Call(t, testHandler.InfrastructureImageResources, imageRequest(t, connection, http.MethodGet, nil, "")).Want(200).JSON(&resources)
	if len(resources.Targets) != 4 || !resources.Configured {
		t.Fatalf("expected two role containers and their sidecars, got %d", len(resources.Targets))
	}
	var info service.ExecutionAccessInfo
	testutil.Call(t, testHandler.InfrastructureExecutionAccess, imageRequest(t, connection, http.MethodGet, nil, "")).Want(200).JSON(&info)
	encoded, _ := json.Marshal(info)
	if strings.Contains(string(encoded), "node-password") || strings.Contains(string(encoded), "private_key") {
		t.Fatal("execution credentials leaked")
	}
	operation := previewImage(t, connection, "foreign")
	if operation.Status != "failed" {
		t.Fatalf("foreign target accepted: %s", operation.Status)
	}
	node.mu.Lock()
	defer node.mu.Unlock()
	if len(node.patched) != 0 {
		t.Fatal("unauthorized or foreign request modified a workload")
	}
}

func TestInfrastructureImagesPinsIndependentRoleDigestsAndSubmitsOnlyOnce(t *testing.T) {
	connection, node := imageFixture(t)
	operation := previewImage(t, connection, "worker", "coordinator")
	if operation.Status != "ready" {
		t.Fatalf("preview failed: %s %s", operation.Status, operation.Error)
	}
	node.mu.Lock()
	if len(node.patched) != 0 {
		t.Fatal("preview modified Kubernetes")
	}
	node.mu.Unlock()
	var plan service.ImageReleasePlan
	if err := json.Unmarshal(operation.Details, &plan); err != nil {
		t.Fatal(err)
	}
	if len(plan.Items) != 2 || plan.Items[0].Image.Digest == plan.Items[1].Image.Digest {
		t.Fatal("roles did not resolve independent images")
	}
	testutil.Call(t, testHandler.StartInfrastructureImages, imageRequest(t, connection, http.MethodPost, nil, operation.ID)).Want(202)
	operation = awaitImageOperation(t, connection, operation.ID)
	if operation.Status != "succeeded" {
		t.Fatalf("publication failed: %s %s", operation.Status, operation.Error)
	}
	testutil.Call(t, testHandler.StartInfrastructureImages, imageRequest(t, connection, http.MethodPost, nil, operation.ID)).Want(409)
	node.mu.Lock()
	defer node.mu.Unlock()
	if len(node.patched) != 2 {
		t.Fatalf("expected one write per workload, got %d", len(node.patched))
	}
	for _, workload := range node.workloads {
		containers := workload["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["containers"].([]any)
		if !strings.Contains(containers[0].(map[string]any)["image"].(string), "@sha256:") {
			t.Fatal("role image was not pinned")
		}
		if containers[1].(map[string]any)["image"] != "manager.example:5000/team/metrics:keep" {
			t.Fatal("unselected sidecar changed")
		}
	}
}

func TestInfrastructureImagesRejectsChangedDeploymentBeforeAnyWrite(t *testing.T) {
	connection, node := imageFixture(t)
	operation := previewImage(t, connection, "worker")
	if operation.Status != "ready" {
		t.Fatal(operation.Error)
	}
	node.mu.Lock()
	node.workloads["worker-a"]["metadata"].(map[string]any)["generation"] = float64(8)
	node.mu.Unlock()
	testutil.Call(t, testHandler.StartInfrastructureImages, imageRequest(t, connection, http.MethodPost, nil, operation.ID)).Want(202)
	operation = awaitImageOperation(t, connection, operation.ID)
	if operation.Status != "conflict" {
		t.Fatalf("changed deployment was accepted: %s %s", operation.Status, operation.Error)
	}
	node.mu.Lock()
	defer node.mu.Unlock()
	if len(node.patched) != 0 {
		t.Fatal("conflict reached Kubernetes patch")
	}
}

func TestInfrastructureImagesRetainsPartialAndUncertainOutcomes(t *testing.T) {
	for _, uncertain := range []bool{false, true} {
		t.Run(fmt.Sprint(uncertain), func(t *testing.T) {
			connection, node := imageFixture(t)
			node.failRollout = "worker-a"
			node.uncertainPatch = uncertain
			operation := previewImage(t, connection, "worker", "coordinator")
			if operation.Status != "ready" {
				t.Fatal(operation.Error)
			}
			testutil.Call(t, testHandler.StartInfrastructureImages, imageRequest(t, connection, http.MethodPost, nil, operation.ID)).Want(202)
			operation = awaitImageOperation(t, connection, operation.ID)
			want := "verification_required"
			if uncertain {
				want = "unknown"
			}
			if operation.Status != want {
				t.Fatalf("got %s, want %s: %s", operation.Status, want, operation.Error)
			}
			node.mu.Lock()
			defer node.mu.Unlock()
			if len(node.patched) != 1 {
				t.Fatal("failed or uncertain submission was retried or continued to another role")
			}
		})
	}
}

func TestInfrastructureImagesReverificationDoesNotResubmitTheWrite(t *testing.T) {
	connection, node := imageFixture(t)
	node.failRollout = "worker-a"
	operation := previewImage(t, connection, "worker")
	testutil.Call(t, testHandler.StartInfrastructureImages, imageRequest(t, connection, http.MethodPost, nil, operation.ID)).Want(202)
	operation = awaitImageOperation(t, connection, operation.ID)
	if operation.Status != "verification_required" {
		t.Fatalf("unexpected initial outcome: %s", operation.Status)
	}
	node.mu.Lock()
	node.failRollout = ""
	node.mu.Unlock()
	operation = awaitImageOperation(t, connection, operation.ID)
	if operation.Status != "succeeded" {
		t.Fatalf("recovered Pod was not verified: %s %s", operation.Status, operation.Error)
	}
	node.mu.Lock()
	defer node.mu.Unlock()
	if len(node.patched) != 1 {
		t.Fatal("verification resubmitted a Kubernetes patch")
	}
}

func TestInfrastructureImagesPatchesSelectedContainersInOneWorkloadAtomically(t *testing.T) {
	connection, node := imageFixture(t)
	input := service.ImagePreviewInput{Action: "publish_images", Items: []service.ImageSelection{{Key: "deployment/qa/worker-a/worker", Source: "harbor.example/team/worker:fix"}, {Key: "deployment/qa/worker-a/metrics", Source: "harbor.example/team/metrics:separate-fix"}}}
	var operation service.InfrastructureOperation
	testutil.Call(t, testHandler.PreviewInfrastructureImages, imageRequest(t, connection, http.MethodPost, input, "")).Want(202).JSON(&operation)
	operation = awaitImageOperation(t, connection, operation.ID)
	if operation.Status != "ready" {
		t.Fatal(operation.Error)
	}
	testutil.Call(t, testHandler.StartInfrastructureImages, imageRequest(t, connection, http.MethodPost, nil, operation.ID)).Want(202)
	operation = awaitImageOperation(t, connection, operation.ID)
	if operation.Status != "succeeded" {
		t.Fatalf("multi-container publication failed: %s %s", operation.Status, operation.Error)
	}
	node.mu.Lock()
	defer node.mu.Unlock()
	if len(node.patched) != 1 {
		t.Fatalf("same workload was patched %d times", len(node.patched))
	}
}

func TestInfrastructureImagesRestartsPinnedRoleWithoutTouchingOtherServices(t *testing.T) {
	connection, node := imageFixture(t)
	image := "harbor.example/team/worker@" + imageTestDigest("already-deployed")
	node.mu.Lock()
	template := node.workloads["worker-a"]["spec"].(map[string]any)["template"].(map[string]any)
	template["spec"].(map[string]any)["containers"].([]any)[0].(map[string]any)["image"] = image
	template["metadata"].(map[string]any)["annotations"] = map[string]any{"keep": "yes"}
	node.mu.Unlock()
	var operation service.InfrastructureOperation
	input := service.ImagePreviewInput{Action: "restart_pods", Items: []service.ImageSelection{{Key: "deployment/qa/worker-a/worker"}}, Reason: "restart selected role"}
	testutil.Call(t, testHandler.PreviewInfrastructureImages, imageRequest(t, connection, http.MethodPost, input, "")).Want(202).JSON(&operation)
	operation = awaitImageOperation(t, connection, operation.ID)
	if operation.Status != "ready" {
		t.Fatal(operation.Error)
	}
	testutil.Call(t, testHandler.StartInfrastructureImages, imageRequest(t, connection, http.MethodPost, nil, operation.ID)).Want(202)
	operation = awaitImageOperation(t, connection, operation.ID)
	if operation.Status != "succeeded" {
		t.Fatalf("restart failed: %s %s", operation.Status, operation.Error)
	}
	node.mu.Lock()
	defer node.mu.Unlock()
	if len(node.patched) != 1 || node.patched[0] != "worker-a" {
		t.Fatalf("unexpected restarted workloads: %v", node.patched)
	}
	annotations := template["metadata"].(map[string]any)["annotations"].(map[string]any)
	if annotations["keep"] != "yes" || annotations["multica.ai/restartedAt"] != operation.ID {
		t.Fatal("restart did not preserve annotations and request a fresh Pod template")
	}
	containers := template["spec"].(map[string]any)["containers"].([]any)
	if containers[0].(map[string]any)["image"] != image || containers[1].(map[string]any)["image"] != "manager.example:5000/team/metrics:keep" {
		t.Fatal("restart changed pinned role or unselected sidecar image")
	}
}

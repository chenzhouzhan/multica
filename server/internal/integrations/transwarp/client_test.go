package transwarp

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func managerFixture(t *testing.T, name string, stoppable bool) (*Client, *atomic.Int32) {
	t.Helper()
	restarts := &atomic.Int32{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/users/login" {
			var input map[string]string
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input["userName"] != name || input["userPassword"] != "test-password" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			http.SetCookie(w, &http.Cookie{Name: "session", Value: name, Path: "/"})
			_, _ = w.Write([]byte(`{}`))
			return
		}
		cookie, err := r.Cookie("session")
		if err != nil || cookie.Value != name {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/users/profile":
			_ = json.NewEncoder(w).Encode(map[string]string{"userName": name})
		case "/api/services":
			_ = json.NewEncoder(w).Encode([]any{map[string]any{"id": 8, "name": "App", "type": "APP", "typeFriendlyName": "Application", "clusterId": 7, "version": "app-v1", "productInfo": map[string]string{"type": "STUDIO", "friendlyName": "Studio", "version": "v1", "displayVersion": "Version 1"}, "dependencies": []int64{1}, "externalDependencies": []string{"external-db"}}})
		case "/api/services/8/dependencies":
			_ = json.NewEncoder(w).Encode([]any{map[string]any{"id": 1, "name": "TOS", "clusterId": nil, "health": "HEALTHY"}})
		case "/api/services/8/dependents":
			_ = json.NewEncoder(w).Encode([]any{map[string]any{"id": 9, "name": "Dependent", "clusterId": 7, "dependencies": []int64{8}}})
		case "/api/serviceRoles", "/api/serviceRoles/11":
			role := map[string]any{"id": 11, "serviceId": 8, "roleType": "WORKER", "installed": true, "frontendStatus": map[string]string{"name": "RUNNING", "title": "Running"}, "health": nil, "node": map[string]any{"id": 1, "hostName": name, "clusterId": 7}}
			if r.URL.Path == "/api/serviceRoles" {
				_ = json.NewEncoder(w).Encode([]any{role})
			} else {
				_ = json.NewEncoder(w).Encode(role)
			}
		case "/api/nodes":
			if r.URL.Query().Get("clusterId") != "7" {
				t.Error("cluster scope was omitted")
			}
			_ = json.NewEncoder(w).Encode([]map[string]any{{"id": 1, "hostName": name, "clusterId": 7, "ipAddress": "10.0.0.1", "repos": []map[string]string{{"proxyPassword": "do-not-return"}}}})
		case "/api/serviceRoles/operations/canStop":
			if r.URL.Query().Get("roleIds") != "11" {
				t.Error("role scope was omitted")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"stoppable": stoppable, "message": "busy"})
		case "/api/serviceRoles/operations/restart":
			var ids []int64
			if err := json.NewDecoder(r.Body).Decode(&ids); err != nil || len(ids) != 1 || ids[0] != 11 {
				t.Error("restart did not target exactly one role")
			}
			restarts.Add(1)
			_, _ = w.Write([]byte(`{"id":29}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	client, err := New(Config{URL: server.URL, Username: name, Password: "test-password", CAPEM: string(cert)})
	if err != nil {
		t.Fatal(err)
	}
	return client, restarts
}

func TestManagersWithTheSameResourceIDsKeepSeparateSessions(t *testing.T) {
	a, _ := managerFixture(t, "manager-a", true)
	b, _ := managerFixture(t, "manager-b", true)
	for _, tc := range []struct {
		client *Client
		name   string
	}{{a, "manager-a"}, {b, "manager-b"}, {a, "manager-a"}} {
		nodes, err := tc.client.Nodes(context.Background(), 7)
		if err != nil {
			t.Fatal(err)
		}
		if len(nodes) != 1 || nodes[0].Name != tc.name || nodes[0].ClusterID != 7 {
			t.Fatalf("wrong Manager or cluster: %+v", nodes)
		}
		encoded, _ := json.Marshal(nodes)
		var decoded []map[string]any
		_ = json.Unmarshal(encoded, &decoded)
		if _, exists := decoded[0]["repos"]; exists {
			t.Fatal("upstream credentials escaped the resource projection")
		}
	}
}

func TestRestartChecksManagerPermissionAndReturnsSubmission(t *testing.T) {
	for _, allowed := range []bool{false, true} {
		client, calls := managerFixture(t, "manager", allowed)
		job, err := client.RestartRole(context.Background(), 11)
		if allowed {
			if err != nil || job.ID != 29 || job.Status != "" || calls.Load() != 1 {
				t.Fatalf("submission: job=%+v calls=%d err=%v", job, calls.Load(), err)
			}
		} else if err == nil || calls.Load() != 0 {
			t.Fatal("a denied pre-check must not submit a restart")
		}
	}
}

func TestAuthenticationFailureDoesNotExposeCredentials(t *testing.T) {
	client, _ := managerFixture(t, "manager", true)
	client.password = "wrong-secret"
	_, err := client.Clusters(context.Background())
	if !errors.Is(err, ErrAuthentication) {
		t.Fatalf("want an authentication error, got %v", err)
	}
}

func TestCertificateVerificationCanBeSkippedPerManager(t *testing.T) {
	trusted, _ := managerFixture(t, "manager", true)
	config := Config{URL: trusted.base.String(), Username: "manager", Password: "test-password"}
	strict, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := strict.Nodes(context.Background(), 7); err == nil {
		t.Fatal("untrusted certificate was accepted by default")
	}
	config.SkipTLSVerify = true
	config.CAPEM = "unused while certificate verification is disabled"
	insecure, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	nodes, err := insecure.Nodes(context.Background(), 7)
	if err != nil || len(nodes) != 1 {
		t.Fatalf("explicit certificate bypass failed: nodes=%+v err=%v", nodes, err)
	}
	if _, err := strict.Nodes(context.Background(), 7); err == nil {
		t.Fatal("bypass changed another Manager's verification policy")
	}
	if _, err := trusted.Nodes(context.Background(), 7); err != nil {
		t.Fatalf("custom CA verification stopped working: %v", err)
	}
}

func TestRoleStatusObjectsAreProjectedForListsAndDetails(t *testing.T) {
	client, _ := managerFixture(t, "manager", true)
	roles, err := client.Roles(context.Background(), 8)
	if err != nil || len(roles) != 1 {
		t.Fatalf("could not decode Manager role list: roles=%+v err=%v", roles, err)
	}
	if roles[0].Status != "RUNNING" || roles[0].Health != "UNKNOWN" {
		t.Fatalf("unexpected role status projection: %+v", roles[0])
	}
	role, err := client.Role(context.Background(), 11)
	if err != nil || role.Status != "RUNNING" || role.Health != "UNKNOWN" {
		t.Fatalf("could not decode Manager role detail: role=%+v err=%v", role, err)
	}
}

func TestManagerStatusDecoding(t *testing.T) {
	for _, tc := range []struct{ body, want string }{
		{`"RUNNING"`, "RUNNING"},
		{`{"name":"RUNNING","title":"Running"}`, "RUNNING"},
		{`{"name":"FUTURE_STATE","title":"Future"}`, "FUTURE_STATE"},
		{`null`, "UNKNOWN"},
		{`{}`, "UNKNOWN"},
		{`""`, "UNKNOWN"},
	} {
		t.Run(tc.body, func(t *testing.T) {
			var status managerStatus
			if err := json.Unmarshal([]byte(tc.body), &status); err != nil || string(status) != tc.want {
				t.Fatalf("status=%q err=%v", status, err)
			}
		})
	}
}

func TestResponseErrorsSeparateInvalidJSONFromFieldTypes(t *testing.T) {
	for _, tc := range []struct{ body, want string }{
		{`<html>private upstream error</html>`, "invalid JSON"},
		{`[{"id":"private-value"}]`, "incompatible field"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(tc.body)) }))
			defer server.Close()
			cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
			client, err := New(Config{URL: server.URL, Username: "manager", Password: "password", CAPEM: string(cert)})
			if err != nil {
				t.Fatal(err)
			}
			var out []rawRole
			err = client.request(context.Background(), http.MethodGet, "/api/serviceRoles", nil, nil, &out)
			if !errors.Is(err, ErrResponse) || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "GET /api/serviceRoles") || strings.Contains(err.Error(), "private") {
				t.Fatalf("incorrect or unsafe response diagnostic: %v", err)
			}
		})
	}
}

func TestServiceProductsAndDependencyDirections(t *testing.T) {
	client, _ := managerFixture(t, "manager", true)
	services, err := client.Services(context.Background(), 7)
	if err != nil || len(services) != 1 {
		t.Fatalf("could not read services: %+v err=%v", services, err)
	}
	service := services[0]
	if service.Product == nil || service.Product.Type != "STUDIO" || service.Product.Version != "v1" || service.Product.DisplayVersion != "Version 1" || len(service.Dependencies) != 1 || service.Dependencies[0] != 1 {
		t.Fatalf("lost product identity or dependency: %+v", service)
	}
	relations, err := client.Relations(context.Background(), 8)
	if err != nil || len(relations.Dependencies) != 1 || len(relations.Dependents) != 1 {
		t.Fatalf("could not read relationships: %+v err=%v", relations, err)
	}
	if relations.Dependencies[0].ID != 1 || relations.Dependencies[0].ClusterID != 0 || relations.Dependents[0].ID != 9 {
		t.Fatalf("global scope or dependency direction changed: %+v", relations)
	}
}

package transwarp

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServiceWorkloadsUsesManagerScopedIdentitiesAndDiscardsManifestSecrets(t *testing.T) {
	for _, quoted := range []bool{false, true} {
		t.Run(map[bool]string{false: "yaml", true: "json string"}[quoted], func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/users/login":
					json.NewEncoder(w).Encode(map[string]any{})
				case "/api/users/profile":
					json.NewEncoder(w).Encode(map[string]string{"userName": "ops"})
				case "/api/services/8/tosObjects/kinds":
					json.NewEncoder(w).Encode([]string{"POD", "DEPLOYMENT"})
				case "/api/services/8/tosObjects":
					if r.URL.Query().Get("kind") != "DEPLOYMENT" {
						t.Error("wrong kind")
					}
					json.NewEncoder(w).Encode([]map[string]string{{"name": "worker-a"}})
				case "/api/services/8/tosObjects/config":
					if r.URL.Query().Get("name") != "worker-a" || r.URL.Query().Get("kind") != "DEPLOYMENT" {
						t.Error("lost service object identity")
					}
					manifest := "kind: Deployment\nmetadata:\n  name: worker-a\n  namespace: qa\n  uid: workload-a\nspec:\n  secret: do-not-project\n"
					if quoted {
						json.NewEncoder(w).Encode(manifest)
					} else {
						w.Header().Set("Content-Type", "text/plain")
						w.Write([]byte(manifest))
					}
				default:
					w.WriteHeader(404)
				}
			}))
			defer server.Close()
			certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
			client, err := New(Config{URL: server.URL, Username: "ops", Password: "password", CAPEM: string(certificate)})
			if err != nil {
				t.Fatal(err)
			}
			refs, err := client.ServiceWorkloads(context.Background(), 8)
			if err != nil {
				t.Fatal(err)
			}
			if len(refs) != 1 || refs[0].Namespace != "qa" || refs[0].Name != "worker-a" || refs[0].UID != "workload-a" {
				t.Fatalf("wrong scoped workloads: %+v", refs)
			}
		})
	}
}

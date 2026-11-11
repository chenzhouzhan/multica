package transwarp

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServiceMetainfoReadWriteAndScope(t *testing.T) {
	for _, name := range []string{"manager-a", "manager-b"} {
		t.Run(name, func(t *testing.T) {
			writes := 0
			binary := false
			reject := false
			malformed := false
			const key = "APP/v1/metainfo.yaml"
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/api/users/login":
					http.SetCookie(w, &http.Cookie{Name: "session", Value: name, Path: "/"})
					w.Write([]byte(`{}`))
				case "/api/users/profile":
					json.NewEncoder(w).Encode(map[string]string{"userName": name})
				case "/api/services/8":
					json.NewEncoder(w).Encode(map[string]any{"id": 8, "type": "APP", "version": "v1"})
				case "/api/toolbox/metaEditor/tree/service":
					cookie, err := r.Cookie("session")
					if err != nil || cookie.Value != name || r.URL.Query().Get("type") != "APP" || r.URL.Query().Get("version") != "v1" {
						t.Error("missing Manager session or service version scope")
					}
					json.NewEncoder(w).Encode([]any{
						map[string]any{"key": key, "title": "metainfo.yaml", "type": "INFO_YAML", "isLeaf": true},
						map[string]any{"key": "OTHER/v1/metainfo.yaml", "type": "INFO_YAML", "isLeaf": true},
						map[string]any{"key": "APP/v1/i18n", "type": "I18N_DIR", "isLeaf": false},
					})
				case "/api/toolbox/metaEditor/file/text":
					if r.Method == http.MethodGet {
						if malformed {
							w.Write([]byte(`{"text":true}`))
							return
						}
						query := r.URL.Query()
						if query.Get("category") != "SERVICE" || query.Get("type") != "INFO_YAML" || query.Get("key") != key {
							t.Error("file scope was omitted")
						}
						json.NewEncoder(w).Encode(map[string]any{"text": !binary, "content": "name: " + name})
						return
					}
					var input map[string]string
					if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input["category"] != "SERVICE" || input["type"] != "INFO_YAML" || input["key"] != key || input["content"] != "name: changed\n" {
						t.Error("saved content or service scope changed")
					}
					writes++
					if reject {
						w.WriteHeader(500)
					}
				default:
					w.WriteHeader(404)
				}
			}))
			defer server.Close()
			cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
			client, err := New(Config{URL: server.URL, Username: name, Password: "test-password", CAPEM: string(cert)})
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			files, err := client.ServiceMetaFiles(ctx, 8)
			if err != nil || len(files) != 1 || files[0].Name != "metainfo.yaml" {
				t.Fatalf("files=%+v err=%v", files, err)
			}
			file, err := client.ServiceMetaFile(ctx, 8, key)
			if err != nil || !file.Text || file.Key != key || file.Content != "name: "+name {
				t.Fatalf("file=%+v err=%v", file, err)
			}
			if err := client.SaveServiceMetaFile(ctx, 8, key, "name: changed\n"); err != nil || writes != 1 {
				t.Fatalf("save: writes=%d err=%v", writes, err)
			}
			for _, invalid := range []string{"OTHER/v1/metainfo.yaml", "APP/v2/metainfo.yaml", "APP/v1/i18n", "APP/v1/../secret"} {
				if _, err := client.ServiceMetaFile(ctx, 8, invalid); err == nil {
					t.Fatal("read accepted an unlisted file")
				}
				if err := client.SaveServiceMetaFile(ctx, 8, invalid, "name: changed\n"); err == nil || writes != 1 {
					t.Fatal("write accepted an unlisted file")
				}
			}
			if _, err := client.ServiceMetaFiles(ctx, -1); err == nil {
				t.Fatal("accepted invalid service identity")
			}
			binary = true
			if err := client.SaveServiceMetaFile(ctx, 8, key, "name: changed\n"); err == nil || writes != 1 {
				t.Fatal("saved a binary file")
			}
			binary, reject = false, true
			if err := client.SaveServiceMetaFile(ctx, 8, key, "name: changed\n"); err == nil || writes != 2 {
				t.Fatal("save failure was hidden or retried")
			}
			malformed = true
			if _, err := client.ServiceMetaFile(ctx, 8, key); !errors.Is(err, ErrResponse) {
				t.Fatal("malformed file content was not reported")
			}
			if err := client.SaveServiceMetaFile(ctx, 8, key, "name: changed\n"); !errors.Is(err, ErrResponse) || writes != 2 {
				t.Fatal("malformed file response was accepted for editing")
			}
		})
	}
}

package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInfrastructureCLIReadsAndSavesThroughScopedToolEndpoint(t *testing.T) {
	content := "name: service\n"
	writes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("X-Workspace-ID") != "ws-1" || r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("lost CLI authentication or workspace")
		}
		var input struct {
			ConnectionID string  `json:"connection_id"`
			ServiceID    int64   `json:"service_id"`
			Key          string  `json:"key"`
			Content      *string `json:"content"`
			Reason       string  `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Error(err)
		}
		if input.ConnectionID != "manager-a" || input.ServiceID != 8 {
			t.Error("lost Manager/service scope")
		}
		switch r.URL.Path {
		case "/api/workspaces/ws-1/infrastructure/tools/list_metainfo_files":
			json.NewEncoder(w).Encode([]any{map[string]string{"key": "APP/v1/metainfo.yaml", "name": "metainfo.yaml", "type": "INFO_YAML"}})
		case "/api/workspaces/ws-1/infrastructure/tools/read_metainfo_file":
			if input.Key != "APP/v1/metainfo.yaml" {
				t.Error("lost file key")
			}
			json.NewEncoder(w).Encode(map[string]any{"key": input.Key, "text": true, "content": content})
		case "/api/workspaces/ws-1/infrastructure/tools/save_metainfo_file":
			if input.Key != "APP/v1/metainfo.yaml" || input.Content == nil || input.Reason != "update translation" {
				t.Error("lost save content or reason")
			}
			content = *input.Content
			writes++
			json.NewEncoder(w).Encode(map[string]any{"saved": true, "audit_id": "recorded-change"})
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	setCLITestServerEnv(t, server.URL)
	run := func(stdin string, args ...string) string {
		t.Helper()
		command := newInfrastructureCommand()
		var output bytes.Buffer
		command.SetOut(&output)
		command.SetIn(strings.NewReader(stdin))
		command.SetArgs(args)
		if err := command.Execute(); err != nil {
			t.Fatal(err)
		}
		return output.String()
	}
	list := run("", "metainfo", "list", "--manager", "manager-a", "--service", "8")
	if !strings.Contains(list, "APP/v1/metainfo.yaml") {
		t.Fatalf("file list missing key: %s", list)
	}
	readArgs := []string{"metainfo", "read", "--manager", "manager-a", "--service", "8", "--key", "APP/v1/metainfo.yaml", "--output", "text"}
	if got := run("", readArgs...); got != content {
		t.Fatalf("read changed file bytes: %q", got)
	}
	if got := run("name: 服务\n", "metainfo", "save", "--manager", "manager-a", "--service", "8", "--key", "APP/v1/metainfo.yaml", "--file", "-", "--reason", "update translation"); !strings.Contains(got, "recorded-change") || writes != 1 {
		t.Fatalf("save=%s writes=%d", got, writes)
	}
	if got := run("", readArgs...); got != "name: 服务\n" {
		t.Fatalf("saved content did not round trip: %q", got)
	}
}

func TestInfrastructureCLIRejectsMalformedFileAndDoesNotRetrySave(t *testing.T) {
	saves := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "save_metainfo_file") {
			saves++
			w.WriteHeader(502)
			w.Write([]byte(`{"error":"save unconfirmed; read before retrying"}`))
			return
		}
		w.Write([]byte(`{"key":"APP/v1/metainfo.yaml","text":true}`))
	}))
	defer server.Close()
	setCLITestServerEnv(t, server.URL)
	command := newInfrastructureCommand()
	command.SetArgs([]string{"metainfo", "read", "--manager", "manager-a", "--service", "8", "--key", "APP/v1/metainfo.yaml"})
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "invalid metainfo") {
		t.Fatalf("malformed response: %v", err)
	}
	command = newInfrastructureCommand()
	command.SetIn(strings.NewReader(""))
	command.SetArgs([]string{"metainfo", "save", "--manager", "manager-a", "--service", "8", "--key", "APP/v1/metainfo.yaml", "--file", "-", "--reason", "clear file"})
	if err := command.Execute(); err == nil || saves != 1 {
		t.Fatalf("uncertain save retried: calls=%d err=%v", saves, err)
	}
}

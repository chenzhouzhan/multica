package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInfrastructureImagesCLIListsPreviewsStartsAndPolls(t *testing.T) {
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Connection string                         `json:"connection_id"`
			Cluster    int64                          `json:"cluster_id"`
			Service    int64                          `json:"service_id"`
			Action     string                         `json:"action"`
			Reason     string                         `json:"reason"`
			Operation  string                         `json:"operation_id"`
			Items      []struct{ Key, Source string } `json:"items"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Error(err)
		}
		if input.Connection != "manager-a" || r.Header.Get("Authorization") != "Bearer test-token" || r.Header.Get("X-Workspace-ID") != "ws-1" {
			t.Error("lost task credentials or Manager scope")
		}
		tool := strings.TrimPrefix(r.URL.Path, "/api/workspaces/ws-1/infrastructure/tools/")
		calls = append(calls, tool)
		if tool != "get_operation" && (input.Cluster != 7 || input.Service != 8) {
			t.Error("lost service scope")
		}
		if tool == "list_service_images" {
			w.Write([]byte(`{"configured":true,"targets":[]}`))
			return
		}
		status := "preparing"
		if tool == "preview_service_images" {
			if input.Action != "publish_images" || input.Reason != "publish requested fix" || len(input.Items) != 2 || input.Items[0].Source == input.Items[1].Source || input.Items[0].Key != "deployment/qa/a/worker" {
				t.Error("lost role selections or independent source images")
			}
		} else if tool == "start_service_images" {
			status = "running"
		} else if tool == "get_operation" {
			status = "succeeded"
		} else {
			t.Errorf("unexpected tool %s", tool)
		}
		if tool != "preview_service_images" && input.Operation != "operation-a" {
			t.Error("lost operation ID")
		}
		json.NewEncoder(w).Encode(map[string]any{"id": "operation-a", "connection_id": "manager-a", "cluster_id": 7, "service_id": 8, "kind": "publish_images", "status": status})
	}))
	defer server.Close()
	setCLITestServerEnv(t, server.URL)
	run := func(stdin string, args ...string) string {
		t.Helper()
		cmd := newInfrastructureCommand()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetIn(strings.NewReader(stdin))
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}
	scope := []string{"--manager", "manager-a", "--cluster", "7", "--service", "8"}
	run("", append([]string{"images", "list"}, scope...)...)
	items := `[{"key":"deployment/qa/a/worker","source":"harbor.example/worker:fix_custom"},{"key":"deployment/qa/b/coordinator","source":"harbor.example/coordinator:other_fix"}]`
	run(items, append(append([]string{"images", "preview"}, scope...), "--file", "-", "--reason", "publish requested fix")...)
	run("", append(append([]string{"images", "start"}, scope...), "--operation", "operation-a")...)
	if out := run("", "images", "get", "--manager", "manager-a", "--operation", "operation-a"); !strings.Contains(out, "succeeded") {
		t.Fatal("operation result missing")
	}
	if strings.Join(calls, ",") != "list_service_images,preview_service_images,start_service_images,get_operation" {
		t.Fatalf("unexpected calls: %v", calls)
	}
}

func TestInfrastructureImagesCLIValidatesSelectionsAndNeverRetriesAnUncertainStart(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(502)
		w.Write([]byte(`{"error":"submission unconfirmed"}`))
	}))
	defer server.Close()
	setCLITestServerEnv(t, server.URL)
	base := []string{"images", "preview", "--manager", "manager-a", "--cluster", "7", "--service", "8", "--file", "-", "--reason", "requested change"}
	for _, data := range []string{`[]`, `[{"key":"a"}]`, `[{"key":"a","source":"x"},{"key":"a","source":"y"}]`, `[{"key":"a","soruce":"x"}]`, `[{"key":"a","source":"x"}] {}`} {
		cmd := newInfrastructureCommand()
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))
		cmd.SetIn(strings.NewReader(data))
		cmd.SetArgs(base)
		if err := cmd.Execute(); err == nil {
			t.Fatalf("invalid selections accepted: %s", data)
		}
	}
	if calls != 0 {
		t.Fatal("invalid selections reached the server")
	}
	cmd := newInfrastructureCommand()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"images", "start", "--manager", "manager-a", "--cluster", "7", "--service", "8", "--operation", "operation-a"})
	if err := cmd.Execute(); err == nil || calls != 1 {
		t.Fatalf("start was retried or accepted: calls=%d err=%v", calls, err)
	}
}

func TestInfrastructureImagesCLIRejectsMismatchedOperationResponses(t *testing.T) {
	input := map[string]any{"connection_id": "manager-a", "cluster_id": int64(7), "service_id": int64(8), "operation_id": "operation-a"}
	for _, wrong := range []map[string]any{{"connection_id": "other"}, {"cluster_id": 9}, {"service_id": 9}, {"id": "other"}, {"kind": "restart_role"}, {"status": ""}} {
		response := map[string]any{"id": "operation-a", "connection_id": "manager-a", "cluster_id": 7, "service_id": 8, "kind": "publish_images", "status": "running"}
		for k, v := range wrong {
			response[k] = v
		}
		data, _ := json.Marshal(response)
		if validateInfrastructureImageResponse("start_service_images", input, data) == nil {
			t.Fatalf("mismatched response accepted: %s", data)
		}
	}
}

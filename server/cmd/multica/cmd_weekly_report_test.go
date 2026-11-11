package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWeeklyReportCLIReadAndSave(t *testing.T) {
	var paths []string
	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.RequestURI())
		if r.Method == "PUT" {
			if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
				t.Error(err)
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"revision": 4})
	}))
	defer server.Close()
	setCLITestServerEnv(t, server.URL)
	cmd := newWeeklyReportCommand()
	cmd.SetOut(io.Discard)
	cmd.SetArgs([]string{"get", "--week", "2026-11-09"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	cmd = newWeeklyReportCommand()
	cmd.SetOut(io.Discard)
	cmd.SetIn(strings.NewReader(`{"version_name":"Logits-3.0.0","version_rows":[{"id":"row","title":"Feature","type":"Function","last_progress":"Connector ready","this_plan":"Verify onsite","note":"Priority reduced"}],"onsite_rows":[]}`))
	cmd.SetArgs([]string{"team", "save", "--week", "2026-11-09", "--revision", "3", "--file", "-"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 || paths[0] != "GET /api/weekly-reports/2026-11-09/mine" || paths[1] != "PUT /api/weekly-team-reports/2026-11-09" {
		t.Fatalf("wrong requests: %v", paths)
	}
	if received["expected_revision"] != float64(3) {
		t.Fatalf("lost revision: %v", received)
	}
	row := received["overview"].(map[string]any)["version_rows"].([]any)[0].(map[string]any)
	if row["id"] != "row" || row["last_progress"] != "Connector ready" || row["note"] != "Priority reduced" {
		t.Fatalf("changed progress data: %v", row)
	}
}

func TestWeeklyReportCLIRejectsAmbiguousWrites(t *testing.T) {
	for _, args := range [][]string{
		{"save", "--week", "2026-11-09", "--file", "-"},
		{"get", "--week", "2026-11-10"},
		{"get", "--member", "not-a-uuid"},
	} {
		cmd := newWeeklyReportCommand()
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Fatalf("accepted invalid command: %v", args)
		}
	}
}

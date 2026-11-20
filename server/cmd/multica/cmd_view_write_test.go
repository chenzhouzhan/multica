package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/cli"
)

func setViewWriteFlags(t *testing.T, cmd *cobra.Command, flags map[string]string) {
	t.Helper()
	for name, value := range flags {
		if err := cmd.Flags().Set(name, value); err != nil {
			t.Fatal(err)
		}
	}
}

func TestViewCreateSendsDefinition(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/issue-views" || r.Header.Get("X-Workspace-ID") != "ws-1" {
			t.Errorf("unexpected request: %s %s, workspace=%q", r.Method, r.URL, r.Header.Get("X-Workspace-ID"))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": testViewMineUUID, "name": body["name"], "revision": 1})
	}))
	t.Cleanup(srv.Close)
	setCLITestServerEnv(t, srv.URL)
	query := `{"projectFilters":["` + testViewProjectUUID + `"],"labelFilters":["label-id"],"propertyFilters":{"type-id":["requirement-id"]}}`
	display := `{"viewMode":"table","tableColumns":[{"key":"title"},{"key":"priority"},{"key":"assignee"},{"key":"status"}]}`
	cmd := newViewCreateCmd()
	setViewWriteFlags(t, cmd, map[string]string{"name": "问数 3.1.0 需求", "visibility": "workspace", "query": query, "display": display})
	out, err := captureStdout(t, func() error { return cmd.RunE(cmd, nil) })
	if err != nil {
		t.Fatal(err)
	}
	var wantQuery, wantDisplay any
	_ = json.Unmarshal([]byte(query), &wantQuery)
	_ = json.Unmarshal([]byte(display), &wantDisplay)
	if body["name"] != "问数 3.1.0 需求" || body["scope_type"] != "workspace" || body["visibility"] != "workspace" ||
		!reflect.DeepEqual(body["query"], wantQuery) || !reflect.DeepEqual(body["display"], wantDisplay) {
		t.Fatalf("create body = %#v", body)
	}
	if !strings.Contains(out, testViewMineUUID) || !strings.Contains(out, "问数 3.1.0 需求") {
		t.Fatalf("output = %s", out)
	}
}

func TestViewCreateScopeAndDefaults(t *testing.T) {
	for _, tc := range []struct {
		name  string
		flags map[string]string
		scope string
	}{
		{"workspace", nil, "workspace"},
		{"project", map[string]string{"project": "7777"}, "project"},
		{"my", map[string]string{"scope": "my", "scope-variant": "created"}, "my"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body map[string]any
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodGet && r.URL.Path == "/api/projects":
					_ = json.NewEncoder(w).Encode(map[string]any{"projects": []any{map[string]any{"id": testViewProjectUUID, "title": "问数"}}, "total": 1})
				case r.Method == http.MethodPost && r.URL.Path == "/api/issue-views":
					_ = json.NewDecoder(r.Body).Decode(&body)
					_ = json.NewEncoder(w).Encode(map[string]any{"id": testViewMineUUID})
				default:
					t.Errorf("unexpected request %s %s", r.Method, r.URL)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			t.Cleanup(srv.Close)
			setCLITestServerEnv(t, srv.URL)
			cmd := newViewCreateCmd()
			setViewWriteFlags(t, cmd, tc.flags)
			setViewWriteFlags(t, cmd, map[string]string{"name": "View"})
			if _, err := captureStdout(t, func() error { return cmd.RunE(cmd, nil) }); err != nil {
				t.Fatal(err)
			}
			if body["scope_type"] != tc.scope || body["visibility"] != "private" ||
				!reflect.DeepEqual(body["query"], map[string]any{}) || !reflect.DeepEqual(body["display"], map[string]any{}) {
				t.Fatalf("body = %#v", body)
			}
			if tc.scope == "project" && body["scope_id"] != testViewProjectUUID {
				t.Fatalf("scope_id = %v", body["scope_id"])
			}
			if tc.scope == "my" && body["scope_variant"] != "created" {
				t.Fatalf("scope_variant = %v", body["scope_variant"])
			}
		})
	}
}

func TestViewUpdateReplacesOnlySuppliedFields(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/issue-views":
			_ = json.NewEncoder(w).Encode([]any{map[string]any{"id": testViewMineUUID, "name": "Mine", "scope_type": "workspace", "revision": 7}})
		case r.Method == http.MethodPatch && r.URL.Path == "/api/issue-views/"+testViewMineUUID:
			_ = json.NewDecoder(r.Body).Decode(&body)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": testViewMineUUID, "name": "Renamed", "revision": 8})
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	setCLITestServerEnv(t, srv.URL)
	file := filepath.Join(t.TempDir(), "display.json")
	if err := os.WriteFile(file, []byte(`{"viewMode":"table","sortBy":"title"}`), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := newViewUpdateCmd()
	setViewWriteFlags(t, cmd, map[string]string{"name": "Renamed", "display-file": file, "expected-revision": "7", "scope-variant": "", "query": "{}"})
	out, err := captureStdout(t, func() error { return cmd.RunE(cmd, []string{"mine"}) })
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"name": "Renamed", "expected_revision": float64(7), "scope_variant": "",
		"display": map[string]any{"viewMode": "table", "sortBy": "title"}, "query": map[string]any{},
	}
	if !reflect.DeepEqual(body, want) {
		t.Fatalf("body = %#v, want %#v", body, want)
	}
	if !strings.Contains(out, `"revision": 8`) {
		t.Fatalf("output = %s", out)
	}
}

func TestViewWritesRejectInvalidFlagsBeforeRequests(t *testing.T) {
	for _, tc := range []struct {
		name   string
		update bool
		flags  map[string]string
		want   string
	}{
		{"missing name", false, nil, "--name"},
		{"empty name", false, map[string]string{"name": " "}, "--name"},
		{"long name", false, map[string]string{"name": strings.Repeat("需", 81)}, "--name"},
		{"bad scope", false, map[string]string{"scope": "unknown"}, "--scope"},
		{"missing project", false, map[string]string{"scope": "project"}, "--project"},
		{"project and my", false, map[string]string{"scope": "my", "project": testViewProjectUUID}, "--project"},
		{"my missing variant", false, map[string]string{"scope": "my"}, "--scope-variant"},
		{"bad variant", false, map[string]string{"scope-variant": "created"}, "--scope-variant"},
		{"shared my", false, map[string]string{"scope": "my", "scope-variant": "any", "visibility": "workspace"}, "private"},
		{"bad visibility", false, map[string]string{"visibility": "public"}, "--visibility"},
		{"bad output", false, map[string]string{"output": "yaml"}, "--output"},
		{"query array", false, map[string]string{"query": "[]"}, "--query"},
		{"query null", false, map[string]string{"query": "null"}, "--query"},
		{"invalid json", false, map[string]string{"display": "{"}, "--display"},
		{"empty json", false, map[string]string{"query": ""}, "--query"},
		{"two sources", false, map[string]string{"query": "{}", "query-file": "missing.json"}, "mutually exclusive"},
		{"missing file", false, map[string]string{"query-file": "missing.json"}, "--query-file"},
		{"missing revision", true, map[string]string{"name": "New"}, "--expected-revision"},
		{"zero revision", true, map[string]string{"name": "New", "expected-revision": "0"}, "--expected-revision"},
		{"no changes", true, map[string]string{"expected-revision": "1"}, "at least one"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Errorf("invalid input made a request: %s %s", r.Method, r.URL)
				w.WriteHeader(http.StatusInternalServerError)
			}))
			t.Cleanup(srv.Close)
			setCLITestServerEnv(t, srv.URL)
			cmd := newViewCreateCmd()
			if tc.update {
				cmd = newViewUpdateCmd()
			}
			if tc.name != "missing name" && !tc.update {
				setViewWriteFlags(t, cmd, map[string]string{"name": "View"})
			}
			setViewWriteFlags(t, cmd, tc.flags)
			err := cmd.RunE(cmd, []string{testViewMineUUID})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestViewUpdatePropagatesConflictWithoutRetry(t *testing.T) {
	patches := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{"id": testViewMineUUID, "scope_type": "workspace", "revision": 9})
			return
		}
		patches++
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"view was modified by someone else"}`))
	}))
	t.Cleanup(srv.Close)
	setCLITestServerEnv(t, srv.URL)
	cmd := newViewUpdateCmd()
	setViewWriteFlags(t, cmd, map[string]string{"name": "New", "expected-revision": "7"})
	out, err := captureStdout(t, func() error { return cmd.RunE(cmd, []string{testViewMineUUID}) })
	var apiErr *cli.HTTPError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusConflict || patches != 1 || out != "" {
		t.Fatalf("output=%q error=%v patches=%d", out, err, patches)
	}
}

func TestViewUpdateNameOnlyKeepsDefinition(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{"id": testViewMineUUID, "scope_type": "my", "revision": 1})
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": testViewMineUUID, "name": "Renamed", "revision": 2})
	}))
	t.Cleanup(srv.Close)
	setCLITestServerEnv(t, srv.URL)
	cmd := newViewUpdateCmd()
	setViewWriteFlags(t, cmd, map[string]string{"name": "Renamed", "expected-revision": "1", "output": "table"})
	out, err := captureStdout(t, func() error { return cmd.RunE(cmd, []string{testViewMineUUID}) })
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(body, map[string]any{"name": "Renamed", "expected_revision": float64(1)}) {
		t.Fatalf("rename must leave query, display, visibility, and variant unchanged: %#v", body)
	}
	if !strings.Contains(out, "REVISION") || !strings.Contains(out, "Renamed") {
		t.Fatalf("table output = %s", out)
	}
}

func TestViewUpdateRejectsIncompatibleScopeChanges(t *testing.T) {
	for _, tc := range []struct {
		scope string
		flags map[string]string
	}{
		{"my", map[string]string{"visibility": "workspace"}},
		{"my", map[string]string{"scope-variant": "agents"}},
		{"project", map[string]string{"scope-variant": "assigned"}},
	} {
		t.Run(tc.scope+"/"+fmt.Sprint(tc.flags), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("invalid scope change made a write: %s %s", r.Method, r.URL)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"id": testViewMineUUID, "scope_type": tc.scope})
			}))
			t.Cleanup(srv.Close)
			setCLITestServerEnv(t, srv.URL)
			cmd := newViewUpdateCmd()
			setViewWriteFlags(t, cmd, tc.flags)
			setViewWriteFlags(t, cmd, map[string]string{"expected-revision": "1"})
			if err := cmd.RunE(cmd, []string{testViewMineUUID}); err == nil {
				t.Fatal("want scope validation error")
			}
		})
	}
}

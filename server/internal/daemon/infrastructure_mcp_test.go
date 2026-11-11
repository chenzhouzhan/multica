package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
)

func TestInfrastructureMCPListsAndRoutesNativeTools(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	config, server, err := startTaskInfrastructureMCP(ctx, func(_ context.Context, tool string, input json.RawMessage) (json.RawMessage, error) {
		calls++
		if tool != "restart_role" {
			t.Fatalf("wrong tool %s", tool)
		}
		var args map[string]any
		if err := json.Unmarshal(input, &args); err != nil || args["connection_id"] != "manager-b" {
			t.Fatalf("lost Manager scope: %s", input)
		}
		return nil, errors.New("role not authorized")
	})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	var decoded struct {
		Servers map[string]struct {
			URL string `json:"url"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(config, &decoded); err != nil {
		t.Fatal(err)
	}
	endpoint := decoded.Servers["multica-infrastructure"].URL
	call := func(payload string) map[string]any {
		t.Helper()
		resp, err := http.Post(endpoint, "application/json", bytes.NewBufferString(payload))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var body map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		return body
	}
	list := call(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	result := list["result"].(map[string]any)
	tools := result["tools"].([]any)
	if len(tools) < 9 {
		t.Fatalf("missing native tools: %v", tools)
	}
	response := call(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"infra_restart_role","arguments":{"connection_id":"manager-b","cluster_id":7,"service_id":8,"role_id":9,"reason":"test"}}}`)
	if calls != 1 || response["result"].(map[string]any)["isError"] != true {
		t.Fatalf("call result=%v calls=%d", response, calls)
	}
	call(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"infra_delete_node","arguments":{}}}`)
	if calls != 1 {
		t.Fatal("unknown mutation reached the backend")
	}
	resp, err := http.Post(endpoint+"wrong", "application/json", bytes.NewBufferString(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatal("unguessable task URL was not enforced")
	}
}

func TestInfrastructureMCPMetainfoCatalogAndReadWrite(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	content := "name: before\n"
	writes := 0
	config, server, err := startTaskInfrastructureMCP(ctx, func(_ context.Context, tool string, input json.RawMessage) (json.RawMessage, error) {
		var args struct {
			Connection string `json:"connection_id"`
			Service    int64  `json:"service_id"`
			Key        string `json:"key"`
			Content    string `json:"content"`
			Reason     string `json:"reason"`
		}
		if err := json.Unmarshal(input, &args); err != nil || args.Connection != "manager-a" || args.Service != 8 {
			t.Fatal("MCP lost Manager or service scope")
		}
		switch tool {
		case "list_metainfo_files":
			return json.RawMessage(`[{"key":"APP/v1/metainfo.yaml","name":"metainfo.yaml","type":"INFO_YAML"}]`), nil
		case "read_metainfo_file":
			data, _ := json.Marshal(map[string]any{"key": args.Key, "text": true, "content": content})
			return data, nil
		case "save_metainfo_file":
			if args.Key != "APP/v1/metainfo.yaml" || args.Reason != "test edit" {
				t.Fatal("MCP lost file key or reason")
			}
			content = args.Content
			writes++
			return json.RawMessage(`{"saved":true,"audit_id":"audit-record"}`), nil
		default:
			t.Fatalf("unexpected tool %s", tool)
			return nil, nil
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	var decoded struct {
		Servers map[string]struct {
			URL string `json:"url"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(config, &decoded); err != nil {
		t.Fatal(err)
	}
	call := func(method string, params any) map[string]any {
		t.Helper()
		payload, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
		resp, err := http.Post(decoded.Servers["multica-infrastructure"].URL, "application/json", bytes.NewReader(payload))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var body struct {
			Result map[string]any `json:"result"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		return body.Result
	}
	list := call("tools/list", nil)
	names := map[string]bool{}
	for _, tool := range list["tools"].([]any) {
		names[tool.(map[string]any)["name"].(string)] = true
	}
	for _, name := range []string{"infra_list_metainfo_files", "infra_read_metainfo_file", "infra_save_metainfo_file"} {
		if !names[name] {
			t.Fatalf("missing %s", name)
		}
	}
	args := map[string]any{"connection_id": "manager-a", "service_id": 8, "key": "APP/v1/metainfo.yaml", "content": "name: after\n", "reason": "test edit"}
	for _, name := range []string{"infra_list_metainfo_files", "infra_read_metainfo_file", "infra_save_metainfo_file", "infra_read_metainfo_file"} {
		result := call("tools/call", map[string]any{"name": name, "arguments": args})
		if result["isError"] != false {
			t.Fatalf("tool failed: %v", result)
		}
		if name == "infra_read_metainfo_file" && writes == 1 {
			text := result["content"].([]any)[0].(map[string]any)["text"].(string)
			var file struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal([]byte(text), &file); err != nil || file.Content != "name: after\n" {
				t.Fatal("MCP read/write round trip failed")
			}
		}
	}
	if writes != 1 {
		t.Fatal("MCP save count changed")
	}
}

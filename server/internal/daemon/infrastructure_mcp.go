package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/pkg/infrastructurecontract"
)

type infrastructureInvoker func(context.Context, string, json.RawMessage) (json.RawMessage, error)

// Manager credentials stay on the backend. The local process receives only
// a per-task loopback URL; each call is authorized against its running task.
func startTaskInfrastructureMCP(ctx context.Context, invoke infrastructureInvoker) (json.RawMessage, *http.Server, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, err
	}
	token, err := randomBrokerToken()
	if err != nil {
		listener.Close()
		return nil, nil, err
	}
	path := "/" + token
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path || r.Method != http.MethodPost {
			w.WriteHeader(404)
			return
		}
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			} `json:"params"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20)).Decode(&req); err != nil {
			w.WriteHeader(400)
			return
		}
		var result any
		switch req.Method {
		case "initialize":
			result = map[string]any{"protocolVersion": "2024-11-05", "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]string{"name": "multica-infrastructure", "version": "1"}}
		case "notifications/initialized":
			w.WriteHeader(202)
			return
		case "tools/list":
			result = map[string]any{"tools": infrastructurecontract.Tools()}
		case "tools/call":
			name := strings.TrimPrefix(req.Params.Name, "infra_")
			var output json.RawMessage
			var callErr error
			if req.Params.Name != "infra_"+name || !infrastructurecontract.Known(name) {
				callErr = fmt.Errorf("unknown infrastructure tool")
			} else {
				callCtx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
				defer cancel()
				output, callErr = invoke(callCtx, name, req.Params.Arguments)
			}
			message := string(output)
			if callErr != nil {
				message = callErr.Error()
			}
			result = map[string]any{"isError": callErr != nil, "content": []map[string]string{{"type": "text", "text": message}}}
		default:
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "error": map[string]any{"code": -32601, "message": "unsupported method"}})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	})
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	go server.Serve(listener)
	go func() {
		select {
		case <-ctx.Done():
			server.Close()
		}
	}()
	config, err := json.Marshal(map[string]any{"mcpServers": map[string]any{"multica-infrastructure": map[string]string{"type": "http", "url": "http://" + listener.Addr().String() + path}}})
	return config, server, err
}

func (c *Client) InvokeInfrastructureTool(ctx context.Context, token, taskID, tool string, input json.RawMessage) (json.RawMessage, error) {
	if len(input) == 0 {
		input = json.RawMessage(`{}`)
	}
	var output json.RawMessage
	err := c.postJSONWithToken(ctx, "/api/daemon/tasks/"+taskID+"/infrastructure/tools/"+tool, token, input, &output)
	return output, err
}

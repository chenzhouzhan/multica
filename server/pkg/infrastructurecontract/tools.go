// Package infrastructurecontract defines the native, server-mediated operations tools.
package infrastructurecontract

import "strings"

type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

func Tools() []Tool {
	definitions := []struct {
		name, description string
		fields            []string
	}{
		{"list_managers", "List the workspace's Manager connections. Always choose a connection before addressing numeric resource IDs.", nil},
		{"list_clusters", "List clusters belonging to one Manager.", []string{"connection_id"}},
		{"list_nodes", "List nodes of one cluster in one Manager.", []string{"connection_id", "cluster_id"}},
		{"list_services", "List services of one cluster in one Manager.", []string{"connection_id", "cluster_id"}},
		{"service_relations", "Read a service’s dependencies and dependents, including global and cross-cluster services in the same Manager.", []string{"connection_id", "service_id"}},
		{"list_roles", "List roles of one service in one Manager.", []string{"connection_id", "service_id"}},
		{"list_metainfo_files", "List metainfo files for a service. Paths and keys belong to that Manager and service version; use the returned key when reading or saving.", []string{"connection_id", "service_id"}},
		{"read_metainfo_file", "Read one service metainfo text file using its listed key. YAML and properties contents are returned verbatim.", []string{"connection_id", "service_id", "key"}},
		{"save_metainfo_file", "Save one service metainfo text file after reading it. Requires a workspace owner/admin human originator and a reason. Changes affect all instances of this service type/version in the chosen Manager. Read the file again to verify; do not automatically retry an uncertain save.", []string{"connection_id", "service_id", "key", "content", "reason"}},
		{"search_logs", "Read logs for a service and host. Defaults to the last 15 minutes; at most 24 hours.", []string{"connection_id", "service_id", "hostname", "level"}},
		{"node_metrics", "Read named node metrics. Discover valid metric names from Manager; do not invent names.", []string{"connection_id", "node_id", "metric_names"}},
		{"restart_role", "Restart exactly one role with prior administrator authorization. Requires connection, cluster, service, role and reason. Submission is not success. Poll get_operation; never retry an uncertain submission automatically.", []string{"connection_id", "cluster_id", "service_id", "role_id", "reason"}},
		{"list_operations", "List recent audited restart operations for one Manager.", []string{"connection_id"}},
		{"get_operation", "Poll a local restart operation and verify the resulting role health. Use the local operation id, not the Manager job id.", []string{"connection_id", "operation_id"}},
	}
	tools := make([]Tool, 0, len(definitions))
	for _, d := range definitions {
		properties := map[string]any{}
		for _, field := range d.fields {
			typ := "string"
			if strings.HasSuffix(field, "_id") && field != "connection_id" && field != "operation_id" {
				typ = "integer"
			}
			properties[field] = map[string]any{"type": typ}
		}
		if d.name == "search_logs" {
			for _, field := range []string{"keywords", "start", "end"} {
				typ := "string"
				if field != "keywords" {
					typ = "integer"
				}
				properties[field] = map[string]any{"type": typ}
			}
			properties["level"] = map[string]any{"type": "string", "enum": []string{"FATAL", "ERROR", "WARN", "INFO", "DEBUG", "TRACE"}}
		}
		required := d.fields
		if required == nil {
			required = []string{}
		}
		tools = append(tools, Tool{"infra_" + d.name, d.description, map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}})
	}
	return tools
}

func Known(name string) bool {
	for _, t := range Tools() {
		if t.Name == "infra_"+name {
			return true
		}
	}
	return false
}

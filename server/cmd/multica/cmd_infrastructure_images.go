package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func addInfrastructureImagesCommands(root *cobra.Command) {
	images := &cobra.Command{Use: "images", Short: "Read role images, prepare a publication or Pod restart, and execute its preview"}
	root.AddCommand(images)
	for _, definition := range []struct{ name, tool, help string }{
		{"list", "list_service_images", "List actual images and target keys for a service"},
		{"preview", "preview_service_images", "Resolve selected images to immutable versions without updating deployments"},
		{"start", "start_service_images", "Execute this task's ready preview once"},
		{"get", "get_operation", "Poll preview, rollout and image verification results"},
	} {
		tool := definition.tool
		cmd := &cobra.Command{Use: definition.name, Short: definition.help, Args: exactArgs(0), RunE: func(cmd *cobra.Command, _ []string) error { return runInfrastructure(cmd, tool) }}
		cmd.Flags().String("output", "json", "Output format: json")
		cmd.Flags().String("manager", "", "Manager connection UUID (required)")
		if tool != "get_operation" {
			cmd.Flags().Int64("cluster", 0, "Cluster ID in this Manager (required)")
			cmd.Flags().Int64("service", 0, "Service ID in this cluster (required)")
		}
		if tool == "preview_service_images" {
			cmd.Flags().String("file", "", "JSON array of {key, source} selections; - reads stdin (required)")
			cmd.Flags().String("action", "publish_images", "publish_images or restart_pods; restart selections only need key")
			cmd.Flags().String("reason", "", "Reason for the change (required)")
		}
		if tool == "start_service_images" || tool == "get_operation" {
			cmd.Flags().String("operation", "", "Local operation UUID from the preview (required)")
		}
		images.AddCommand(cmd)
	}
}

func populateInfrastructureImageInput(cmd *cobra.Command, tool string, input map[string]any) error {
	if tool != "get_operation" {
		cluster, _ := cmd.Flags().GetInt64("cluster")
		service, _ := cmd.Flags().GetInt64("service")
		if cluster <= 0 || service <= 0 {
			return errors.New("--cluster and --service must be positive IDs")
		}
		input["cluster_id"], input["service_id"] = cluster, service
	}
	if tool == "start_service_images" || tool == "get_operation" {
		id, _ := cmd.Flags().GetString("operation")
		if strings.TrimSpace(id) == "" {
			return errors.New("--operation is required; use the local UUID returned by preview")
		}
		input["operation_id"] = id
	}
	if tool != "preview_service_images" {
		return nil
	}
	action, _ := cmd.Flags().GetString("action")
	reason, _ := cmd.Flags().GetString("reason")
	path, _ := cmd.Flags().GetString("file")
	if (action != "publish_images" && action != "restart_pods") || strings.TrimSpace(reason) == "" || len(reason) > 2000 || path == "" {
		return errors.New("--file, --reason (1–2000 bytes) and --action publish_images or restart_pods are required")
	}
	reader := cmd.InOrStdin()
	if path != "-" {
		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("read image selections: %w", err)
		}
		defer file.Close()
		reader = file
	}
	data, err := io.ReadAll(io.LimitReader(reader, (64<<10)+1))
	if err != nil {
		return err
	}
	if len(data) > 64<<10 {
		return errors.New("image selections must not exceed 64 KiB")
	}
	var items []struct {
		Key    string `json:"key"`
		Source string `json:"source,omitempty"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&items); err != nil {
		return fmt.Errorf("read image selections JSON: %w", err)
	}
	if decoder.Decode(new(any)) != io.EOF || len(items) == 0 || len(items) > 16 {
		return errors.New("image selections must be one JSON array containing 1–16 items")
	}
	seen := map[string]bool{}
	for _, item := range items {
		if strings.TrimSpace(item.Key) == "" || seen[item.Key] || (action == "publish_images" && strings.TrimSpace(item.Source) == "") {
			return errors.New("select distinct listed target keys and provide a source for each published image")
		}
		seen[item.Key] = true
	}
	input["action"], input["items"], input["reason"] = action, items, strings.TrimSpace(reason)
	return nil
}

func validateInfrastructureImageResponse(tool string, input map[string]any, data json.RawMessage) error {
	if tool == "list_service_images" {
		var resources struct {
			Configured *bool              `json:"configured"`
			Targets    *[]json.RawMessage `json:"targets"`
		}
		if json.Unmarshal(data, &resources) != nil || resources.Configured == nil || resources.Targets == nil {
			return errors.New("invalid service image resources response")
		}
		return nil
	}
	var operation struct {
		ID         string `json:"id"`
		Connection string `json:"connection_id"`
		Cluster    int64  `json:"cluster_id"`
		Service    int64  `json:"service_id"`
		Kind       string `json:"kind"`
		Status     string `json:"status"`
	}
	if json.Unmarshal(data, &operation) != nil || operation.ID == "" || operation.Status == "" || operation.Connection != input["connection_id"] || (operation.Kind != "publish_images" && operation.Kind != "restart_pods") {
		return errors.New("invalid image operation response; inspect the operation before retrying")
	}
	if tool != "get_operation" && (operation.Cluster != input["cluster_id"] || operation.Service != input["service_id"]) {
		return errors.New("image operation response belongs to another cluster or service; inspect before retrying")
	}
	if tool == "preview_service_images" && operation.Kind != input["action"] {
		return errors.New("image preview response has a different action; inspect before retrying")
	}
	if id, ok := input["operation_id"]; ok && operation.ID != id {
		return errors.New("image operation response has a different operation ID; inspect before retrying")
	}
	return nil
}

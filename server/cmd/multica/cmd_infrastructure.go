package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/spf13/cobra"
)

var infrastructureCmd = newInfrastructureCommand()

func newInfrastructureCommand() *cobra.Command {
	root := &cobra.Command{Use: "infrastructure", Aliases: []string{"infra"}, Short: "Read Manager resources and read or edit service metainfo"}
	add := func(parent *cobra.Command, name, help, tool string) *cobra.Command {
		cmd := &cobra.Command{Use: name, Short: help, Args: exactArgs(0), RunE: func(cmd *cobra.Command, _ []string) error { return runInfrastructure(cmd, tool) }}
		cmd.Flags().String("output", "json", "Output format: json; metainfo read also supports text")
		if tool != "list_managers" {
			cmd.Flags().String("manager", "", "Manager connection UUID (required)")
		}
		if strings.Contains(tool, "metainfo") {
			cmd.Flags().Int64("service", 0, "Service ID in this Manager (required)")
		}
		parent.AddCommand(cmd)
		return cmd
	}
	add(root, "managers", "List Manager connections in the current workspace", "list_managers")
	add(root, "clusters", "List clusters in a Manager", "list_clusters")
	services := add(root, "services", "List services in a Manager cluster", "list_services")
	services.Flags().Int64("cluster", 0, "Cluster ID (required)")
	meta := &cobra.Command{Use: "metainfo", Short: "List, read and save service metainfo files", Long: "Metainfo is shared by all instances of the service type/version in the selected Manager. Read before editing and read again after saving to verify."}
	root.AddCommand(meta)
	add(meta, "list", "List a service's metainfo files and their keys", "list_metainfo_files")
	read := add(meta, "read", "Read a metainfo file using the key returned by list", "read_metainfo_file")
	read.Flags().String("key", "", "Metainfo file key (required)")
	save := add(meta, "save", "Save a text file as a workspace owner/admin or their running agent", "save_metainfo_file")
	save.Flags().String("key", "", "Metainfo file key (required)")
	save.Flags().String("file", "", "Local UTF-8 content file, or - for stdin (required)")
	save.Flags().String("reason", "", "Reason for the change (required)")
	return root
}

func runInfrastructure(cmd *cobra.Command, tool string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	if client.WorkspaceID == "" {
		return errors.New("workspace ID is required")
	}
	output, _ := cmd.Flags().GetString("output")
	if output != "json" && !(tool == "read_metainfo_file" && output == "text") {
		return errors.New("output must be json, or text for metainfo read")
	}
	input := map[string]any{}
	if tool != "list_managers" {
		manager, _ := cmd.Flags().GetString("manager")
		if strings.TrimSpace(manager) == "" {
			return errors.New("--manager is required; discover connection UUIDs with infrastructure managers")
		}
		input["connection_id"] = manager
	}
	if tool == "list_services" {
		cluster, _ := cmd.Flags().GetInt64("cluster")
		if cluster <= 0 {
			return errors.New("--cluster must be a positive ID")
		}
		input["cluster_id"] = cluster
	}
	if strings.Contains(tool, "metainfo") {
		service, _ := cmd.Flags().GetInt64("service")
		if service <= 0 {
			return errors.New("--service must be a positive ID")
		}
		input["service_id"] = service
	}
	if tool == "read_metainfo_file" || tool == "save_metainfo_file" {
		key, _ := cmd.Flags().GetString("key")
		if key == "" {
			return errors.New("--key is required; use a key returned by metainfo list")
		}
		input["key"] = key
	}
	if tool == "save_metainfo_file" {
		path, _ := cmd.Flags().GetString("file")
		reason, _ := cmd.Flags().GetString("reason")
		if path == "" || strings.TrimSpace(reason) == "" || len(reason) > 2000 {
			return errors.New("--file and a --reason of at most 2000 bytes are required")
		}
		reader := cmd.InOrStdin()
		if path != "-" {
			file, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("read content file: %w", err)
			}
			defer file.Close()
			reader = file
		}
		content, err := io.ReadAll(io.LimitReader(reader, (1<<20)+1))
		if err != nil {
			return err
		}
		if len(content) > 1<<20 {
			return errors.New("metainfo content must not exceed 1 MiB")
		}
		if !utf8.Valid(content) {
			return errors.New("metainfo content must be UTF-8 text")
		}
		input["content"], input["reason"] = string(content), reason
	}
	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()
	var result json.RawMessage
	if err := client.PostJSON(ctx, "/api/workspaces/"+client.WorkspaceID+"/infrastructure/tools/"+tool, input, &result); err != nil {
		return err
	}
	if tool == "read_metainfo_file" {
		var file struct {
			Key     string  `json:"key"`
			Text    *bool   `json:"text"`
			Content *string `json:"content"`
		}
		if err := json.Unmarshal(result, &file); err != nil || file.Key != input["key"] || file.Text == nil || file.Content == nil {
			return errors.New("invalid metainfo file response")
		}
		if output == "text" {
			if !*file.Text {
				return errors.New("metainfo file is binary")
			}
			_, err := fmt.Fprint(cmd.OutOrStdout(), *file.Content)
			return err
		}
	}
	if tool == "save_metainfo_file" {
		var saved struct {
			Saved   bool   `json:"saved"`
			AuditID string `json:"audit_id"`
		}
		if err := json.Unmarshal(result, &saved); err != nil || !saved.Saved || saved.AuditID == "" {
			return errors.New("invalid metainfo save response; read the file before retrying")
		}
	}
	return cli.PrintJSON(cmd.OutOrStdout(), result)
}

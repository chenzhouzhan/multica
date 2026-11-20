package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/cli"
)

func newViewCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a saved issue view",
		Long: `Create a saved issue view. Defaults to a private workspace view.

--project selects a project view and accepts a project ID or ID prefix.
--scope my requires --scope-variant assigned, created, involved, or any;
my views are always private.

--query and --display take JSON objects in the same shape returned by
"view get". Use --query-file and --display-file to read UTF-8 JSON files.
For example, --query '{"priorityFilters":["high"]}' filters by priority,
and --display '{"viewMode":"table"}' selects a table. Resolve project,
label, property, and option references to IDs before using them in JSON.`,
		Args: exactArgs(0),
		RunE: runViewCreate,
	}
	addViewWriteFlags(cmd)
	cmd.Flags().String("scope", "workspace", "View scope: workspace, my, or project")
	cmd.Flags().String("project", "", "Project ID or ID prefix (implies --scope project)")
	return cmd
}

func newViewUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <view>",
		Short: "Update a saved issue view",
		Long: `Update a saved issue view by ID, ID prefix, or exact name.

Only supplied fields change. --query and --display each replace the entire
saved object; use '{}' to clear one. Use --query-file and --display-file
to read UTF-8 JSON files in the same shape returned by "view get".

Read the view with "view get" first and pass its revision through
--expected-revision. A concurrent edit returns a conflict; read the view
again and reconcile your changes before retrying. Scope and project cannot
be changed after creation. Owners can edit their views; workspace admins
can also edit workspace-shared views.`,
		Args: exactArgs(1),
		RunE: runViewUpdate,
	}
	addViewWriteFlags(cmd)
	cmd.Flags().Int32("expected-revision", 0, "Revision from view get (required)")
	return cmd
}

func addViewWriteFlags(cmd *cobra.Command) {
	cmd.Flags().String("name", "", "View name (required for create, 1–80 characters)")
	cmd.Flags().String("visibility", "private", "Visibility: private or workspace")
	cmd.Flags().String("scope-variant", "", "Scope variant: all, members, agents; for my: assigned, created, involved, any")
	cmd.Flags().String("query", "", "Filter definition as a JSON object; replaces all saved filters")
	cmd.Flags().String("query-file", "", "Read the filter definition from a UTF-8 JSON file")
	cmd.Flags().String("display", "", "Display configuration as a JSON object; replaces all saved display settings")
	cmd.Flags().String("display-file", "", "Read the display configuration from a UTF-8 JSON file")
	cmd.Flags().String("output", "json", "Output format: table or json")
}

func readViewObjectFlag(cmd *cobra.Command, name string) (json.RawMessage, bool, error) {
	inline := cmd.Flags().Changed(name)
	fromFile := cmd.Flags().Changed(name + "-file")
	if inline && fromFile {
		return nil, false, fmt.Errorf("--%s and --%s-file are mutually exclusive", name, name)
	}
	if !inline && !fromFile {
		return nil, false, nil
	}
	raw, _ := cmd.Flags().GetString(name)
	if fromFile {
		path, _ := cmd.Flags().GetString(name + "-file")
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, false, fmt.Errorf("read --%s-file: %w", name, err)
		}
		raw = string(data)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &object); err != nil || object == nil {
		return nil, false, fmt.Errorf("--%s must be a JSON object", name)
	}
	return json.RawMessage(raw), true, nil
}

func viewWriteBody(cmd *cobra.Command, create bool) (map[string]any, error) {
	output, _ := cmd.Flags().GetString("output")
	if output != "json" && output != "table" {
		return nil, fmt.Errorf("invalid --output %q; valid values: table, json", output)
	}
	body := map[string]any{}
	if create || cmd.Flags().Changed("name") {
		name, _ := cmd.Flags().GetString("name")
		if strings.TrimSpace(name) == "" || utf8.RuneCountInString(name) > 80 {
			return nil, fmt.Errorf("--name is required and must be between 1 and 80 characters")
		}
		body["name"] = name
	}
	if create || cmd.Flags().Changed("visibility") {
		visibility, _ := cmd.Flags().GetString("visibility")
		if visibility != "private" && visibility != "workspace" {
			return nil, fmt.Errorf("invalid --visibility %q; valid values: private, workspace", visibility)
		}
		body["visibility"] = visibility
	}
	if cmd.Flags().Changed("scope-variant") {
		variant, _ := cmd.Flags().GetString("scope-variant")
		body["scope_variant"] = variant
	}
	for _, name := range []string{"query", "display"} {
		object, supplied, err := readViewObjectFlag(cmd, name)
		if err != nil {
			return nil, err
		}
		if supplied {
			body[name] = object
		} else if create {
			body[name] = json.RawMessage("{}")
		}
	}
	return body, nil
}

func validateViewWriteScope(scope string, body map[string]any, create bool) error {
	if scope == "my" {
		if visibility, ok := body["visibility"]; ok && visibility != "private" {
			return fmt.Errorf("my views are always private")
		}
	}
	variant, supplied := body["scope_variant"].(string)
	if !supplied && (!create || scope != "my") {
		return nil
	}
	valid := []string{"", "all", "members", "agents"}
	if scope == "my" {
		valid = []string{"assigned", "created", "involved", "any"}
	}
	if !slices.Contains(valid, variant) {
		return fmt.Errorf("invalid --scope-variant %q for --scope %s; valid values: %s", variant, scope, strings.Join(valid, ", "))
	}
	return nil
}

func runViewCreate(cmd *cobra.Command, _ []string) error {
	body, err := viewWriteBody(cmd, true)
	if err != nil {
		return err
	}
	scope, _ := cmd.Flags().GetString("scope")
	project, _ := cmd.Flags().GetString("project")
	if cmd.Flags().Changed("project") {
		if strings.TrimSpace(project) == "" {
			return fmt.Errorf("--project must not be empty")
		}
		if cmd.Flags().Changed("scope") && scope != "project" {
			return fmt.Errorf("--project cannot be combined with --scope %s", scope)
		}
		scope = "project"
	}
	if scope != "workspace" && scope != "project" && scope != "my" {
		return fmt.Errorf("invalid --scope %q; valid values: workspace, my, project", scope)
	}
	if scope == "project" && project == "" {
		return fmt.Errorf("--project is required for project views")
	}
	if err := validateViewWriteScope(scope, body, true); err != nil {
		return err
	}
	body["scope_type"] = scope
	body["definition_version"] = 1
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	if client.WorkspaceID == "" {
		return fmt.Errorf("workspace_id is required to create views")
	}
	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()
	if scope == "project" {
		ref, err := resolveProjectID(ctx, client, project)
		if err != nil {
			return err
		}
		body["scope_id"] = ref.ID
	}
	var result map[string]any
	if err := client.PostJSON(ctx, "/api/issue-views", body, &result); err != nil {
		return fmt.Errorf("create view: %w", err)
	}
	return printViewWriteResult(cmd, result)
}

func runViewUpdate(cmd *cobra.Command, args []string) error {
	revision, _ := cmd.Flags().GetInt32("expected-revision")
	if revision < 1 {
		return fmt.Errorf("--expected-revision is required and must be a positive integer; read it with `multica view get <view>`")
	}
	body, err := viewWriteBody(cmd, false)
	if err != nil {
		return err
	}
	if len(body) == 0 {
		return fmt.Errorf("pass at least one of --name, --visibility, --scope-variant, --query, --query-file, --display, or --display-file")
	}
	body["expected_revision"] = revision
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	if client.WorkspaceID == "" {
		return fmt.Errorf("workspace_id is required to update views")
	}
	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()
	view, err := resolveIssueView(ctx, client, args[0])
	if err != nil {
		return fmt.Errorf("resolve view: %w", err)
	}
	if err := validateViewWriteScope(strVal(view, "scope_type"), body, false); err != nil {
		return err
	}
	var result map[string]any
	if err := client.PatchJSON(ctx, "/api/issue-views/"+url.PathEscape(strVal(view, "id")), body, &result); err != nil {
		return fmt.Errorf("update view: %w", err)
	}
	return printViewWriteResult(cmd, result)
}

func printViewWriteResult(cmd *cobra.Command, view map[string]any) error {
	if output, _ := cmd.Flags().GetString("output"); output == "table" {
		cli.PrintTable(os.Stdout, []string{"ID", "NAME", "SCOPE", "VISIBILITY", "REVISION"}, [][]string{{
			strVal(view, "id"), strVal(view, "name"), viewScopeLabel(view, true),
			strVal(view, "visibility"), fmt.Sprint(view["revision"]),
		}})
		return nil
	}
	return cli.PrintJSON(os.Stdout, view)
}

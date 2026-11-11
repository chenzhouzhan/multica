package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"time"

	"github.com/multica-ai/multica/server/internal/cli"
	wr "github.com/multica-ai/multica/server/pkg/weeklyreport"
	"github.com/spf13/cobra"
)

var weeklyReportCmd = newWeeklyReportCommand()

func newWeeklyReportCommand() *cobra.Command {
	root := &cobra.Command{Use: "weekly-report", Short: "Read and update weekly reports and team progress", Long: "Read and update workspace weekly reports. Agent credentials act for their bound member: personal writes target that member only; team progress writes require owner/admin. Saving changes a draft; submit and team generate are explicit actions."}
	team := &cobra.Command{Use: "team", Short: "Read, update and generate the leader's team report"}
	for _, entry := range []struct {
		parent        *cobra.Command
		action, short string
	}{
		{root, "list", "List submitted reports and your draft for a week"},
		{root, "get", "Get your report, or a member's published report with --member"},
		{root, "history", "List weekly report history"},
		{root, "save", "Replace your draft from a JSON content file"},
		{root, "submit", "Publish your saved draft"},
		{team, "get", "Read team progress and submitted member contributions"},
		{team, "save", "Replace the team's overall progress draft from JSON"},
		{team, "generate", "Snapshot the saved overview and all submitted member reports"},
	} {
		action := entry.action
		isTeam := entry.parent == team
		command := &cobra.Command{Use: action, Short: entry.short, Args: exactArgs(0)}
		command.Flags().String("week", "", "Week's Monday in YYYY-MM-DD (default: current local week)")
		command.Flags().String("output", "json", "Output format: json")
		if action == "get" && !isTeam || action == "history" {
			command.Flags().String("member", "", "Member UUID (default: your own report)")
		}
		if action == "history" {
			command.Flags().String("before", "", "Only weeks before this date (YYYY-MM-DD)")
			command.Flags().Int("limit", 52, "Maximum history entries, 1 to 100")
		}
		if action == "save" {
			command.Flags().String("file", "", "JSON file; use - for stdin (content for personal, overview for team)")
		}
		if action == "save" || action == "submit" || action == "generate" {
			command.Flags().Int32("revision", 0, "Expected revision from get; use 0 for the first save (required)")
		}
		command.RunE = func(cmd *cobra.Command, _ []string) error { return runWeeklyReport(cmd, action, isTeam) }
		entry.parent.AddCommand(command)
	}
	root.AddCommand(team)
	return root
}

func weeklyCLIWeek(cmd *cobra.Command) (string, error) {
	week, _ := cmd.Flags().GetString("week")
	if week == "" {
		now := time.Now()
		day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		day = day.AddDate(0, 0, -(int(day.Weekday())+6)%7)
		week = day.Format(time.DateOnly)
	}
	_, err := wr.Week(week)
	return week, err
}

func weeklyCLIFile(cmd *cobra.Command, out any) error {
	file, _ := cmd.Flags().GetString("file")
	if file == "" {
		return fmt.Errorf("--file is required; use --file - to read JSON from stdin")
	}
	var reader io.Reader = cmd.InOrStdin()
	if file != "-" {
		opened, err := os.Open(file)
		if err != nil {
			return err
		}
		defer opened.Close()
		reader = opened
	}
	decoder := json.NewDecoder(io.LimitReader(reader, (1<<20)+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return fmt.Errorf("read weekly report JSON: %w", err)
	}
	if decoder.Decode(new(any)) != io.EOF {
		return fmt.Errorf("expected a single JSON document")
	}
	return nil
}

func runWeeklyReport(cmd *cobra.Command, action string, team bool) error {
	week, err := weeklyCLIWeek(cmd)
	if err != nil {
		return err
	}
	if output, _ := cmd.Flags().GetString("output"); output != "json" {
		return fmt.Errorf("weekly report output must be json")
	}
	var revision int32
	if action == "save" || action == "submit" || action == "generate" {
		if !cmd.Flags().Changed("revision") {
			return fmt.Errorf("--revision is required; read the report first (0 for a new report)")
		}
		revision, _ = cmd.Flags().GetInt32("revision")
		if revision < 0 {
			return fmt.Errorf("--revision must be non-negative")
		}
	}
	var input any
	if action == "save" {
		if team {
			var overview wr.Overview
			if err := weeklyCLIFile(cmd, &overview); err != nil {
				return err
			}
			if err := overview.Normalize(false); err != nil {
				return err
			}
			input = map[string]any{"overview": overview, "expected_revision": revision}
		} else {
			var content wr.Content
			if err := weeklyCLIFile(cmd, &content); err != nil {
				return err
			}
			if err := content.Normalize(false); err != nil {
				return err
			}
			input = map[string]any{"content": content, "expected_revision": revision}
		}
	}
	path := "/api/weekly-reports/" + url.PathEscape(week) + "/mine"
	if team {
		path = "/api/weekly-team-reports/" + url.PathEscape(week)
	}
	if action == "list" {
		path = "/api/weekly-reports?week=" + url.QueryEscape(week)
	}
	if action == "get" && !team {
		if member, _ := cmd.Flags().GetString("member"); member != "" {
			if !uuidRegexp.MatchString(member) {
				return fmt.Errorf("--member must be a full member UUID")
			}
			path = "/api/weekly-reports/" + url.PathEscape(week) + "/members/" + url.PathEscape(member)
		}
	}
	if action == "history" {
		query := url.Values{}
		if member, _ := cmd.Flags().GetString("member"); member != "" {
			if !uuidRegexp.MatchString(member) {
				return fmt.Errorf("--member must be a full member UUID")
			}
			query.Set("user_id", member)
		}
		if before, _ := cmd.Flags().GetString("before"); before != "" {
			query.Set("before", before)
		}
		limit, _ := cmd.Flags().GetInt("limit")
		if limit < 1 || limit > 100 {
			return fmt.Errorf("--limit must be between 1 and 100")
		}
		query.Set("limit", fmt.Sprint(limit))
		path = "/api/weekly-reports/history?" + query.Encode()
	}
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	if client.WorkspaceID == "" {
		return fmt.Errorf("workspace is required")
	}
	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()
	var result any
	switch action {
	case "save":
		err = client.PutJSON(ctx, path, input, &result)
	case "submit", "generate":
		err = client.PostJSON(ctx, path+"/"+action, map[string]any{"expected_revision": revision}, &result)
	default:
		err = client.GetJSON(ctx, path, &result)
	}
	if err != nil {
		return err
	}
	return cli.PrintJSON(cmd.OutOrStdout(), result)
}

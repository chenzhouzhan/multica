package handler

import (
	"context"
	"encoding/json"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/middleware"
	"net/http"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
	wr "github.com/multica-ai/multica/server/pkg/weeklyreport"
)

func weeklyRequest(method, path string, body any, user string) *http.Request {
	req := testutil.WithHeaders(testutil.JSONRequest(method, path, body), "X-User-ID", user, "X-Workspace-ID", testWorkspaceID)
	return testutil.WithURLParams(req, "week", "2026-11-09")
}

func weeklyCleanup(t *testing.T) {
	t.Helper()
	for _, table := range []string{"weekly_report", "weekly_team_report", "weekly_report_period"} {
		dbfx.Cleanup(t, "DELETE FROM "+table+" WHERE workspace_id = $1", testWorkspaceID)
	}
}

func weeklyContent(title string) wr.Content {
	return wr.Content{Tasks: []wr.Task{{ID: "task-one", Title: title, Completed: "Completed work"}}}
}

func TestWeeklyReportDraftPublishAndAgentAuthority(t *testing.T) {
	weeklyCleanup(t)
	other := dbfx.Insert(t, "user", testutil.Cols{"name": "Other", "email": t.Name() + "@test.local"})
	dbfx.Insert(t, "member", testutil.Cols{"workspace_id": testWorkspaceID, "user_id": other, "role": "member"})
	var saved WeeklyReportResponse
	req := weeklyRequest("PUT", "/api/weekly-reports/2026-11-09/mine", map[string]any{"content": weeklyContent("Original"), "expected_revision": 0}, testUserID)
	req.Header.Set("X-Actor-Source", "task_token")
	req.Header.Set("X-Agent-ID", testUserID)
	testutil.Call(t, testHandler.SaveWeeklyReport, req).Want(200).JSON(&saved)
	if saved.Revision != 1 || saved.UpdatedByType != "agent" || len(saved.Draft) == 0 || len(saved.Published) != 0 {
		t.Fatalf("wrong saved draft: %+v", saved)
	}
	read := weeklyRequest("GET", "/weekly", nil, other)
	read = testutil.WithURLParams(read, "week", "2026-11-09", "userId", testUserID)
	testutil.Call(t, testHandler.GetWeeklyReport, read).Want(404)
	var listed WeeklyReportWeekResponse
	testutil.Call(t, testHandler.ListWeeklyReports, weeklyRequest("GET", "/weekly?week=2026-11-09", nil, other)).Want(200).JSON(&listed)
	if len(listed.Reports) != 0 {
		t.Fatal("another member's draft leaked into the list")
	}
	testutil.Call(t, testHandler.SaveWeeklyReport, weeklyRequest("PUT", "/weekly", map[string]any{"content": weeklyContent("Stale"), "expected_revision": 0}, testUserID)).Want(409)
	testutil.Call(t, testHandler.SubmitWeeklyReport, weeklyRequest("POST", "/weekly/submit", map[string]any{"expected_revision": 1}, testUserID)).Want(200).JSON(&saved)
	var visible WeeklyReportResponse
	testutil.Call(t, testHandler.GetWeeklyReport, read).Want(200).JSON(&visible)
	if len(visible.Draft) != 0 || len(visible.Published) == 0 {
		t.Fatal("published read exposed a draft")
	}
	testutil.Call(t, testHandler.SaveWeeklyReport, weeklyRequest("PUT", "/weekly", map[string]any{"content": weeklyContent("Unpublished edit"), "expected_revision": saved.Revision}, testUserID)).Want(200)
	testutil.Call(t, testHandler.GetWeeklyReport, read).Want(200).JSON(&visible)
	var content wr.Content
	if err := json.Unmarshal(visible.Published, &content); err != nil {
		t.Fatal(err)
	}
	if content.Tasks[0].Title != "Original" {
		t.Fatal("draft edits changed the published report")
	}
	for _, body := range []any{
		map[string]any{"content": weeklyContent("Missing revision")},
		map[string]any{"content": weeklyContent("Impersonation"), "expected_revision": 0, "user_id": other},
		`{"content":{},"expected_revision":0} {"content":{}}`,
	} {
		testutil.Call(t, testHandler.SaveWeeklyReport, weeklyRequest("PUT", "/weekly", body, testUserID)).Want(400)
	}
	var history []WeeklyReportResponse
	testutil.Call(t, testHandler.WeeklyReportHistory, weeklyRequest("GET", "/weekly/history?user_id="+testUserID, nil, other)).Want(200).JSON(&history)
	if len(history) != 1 || len(history[0].Draft) != 0 {
		t.Fatal("history did not preserve published-only access")
	}
}

func TestWeeklyTeamReportPermissionsAndSnapshot(t *testing.T) {
	weeklyCleanup(t)
	other := dbfx.Insert(t, "user", testutil.Cols{"name": "Team Member", "email": t.Name() + "@test.local"})
	dbfx.Insert(t, "member", testutil.Cols{"workspace_id": testWorkspaceID, "user_id": other, "role": "member"})
	overview := wr.Overview{VersionName: "Logits-3.0.0", VersionRows: []wr.Progress{{Title: "Skill configuration", Type: "Feature", LastProgress: "Testing"}}, OnsiteRows: []wr.Progress{{Title: "Site integration", Type: "Customization", ThisPlan: "Verify connector"}}}
	var team WeeklyTeamReportResponse
	request := weeklyRequest("PUT", "/team", map[string]any{"overview": overview, "expected_revision": 0}, testUserID)
	request.Header.Set("X-Actor-Source", "task_token")
	request.Header.Set("X-Agent-ID", testUserID)
	testutil.Call(t, testHandler.SaveWeeklyTeamReport, request).Want(200).JSON(&team)
	if !team.CanManage || len(team.Draft) == 0 {
		t.Fatal("manager agent could not save overall progress")
	}
	testutil.Call(t, testHandler.SaveWeeklyTeamReport, weeklyRequest("PUT", "/team", map[string]any{"overview": overview, "expected_revision": team.Revision}, other)).Want(403)
	var memberView WeeklyTeamReportResponse
	testutil.Call(t, testHandler.GetWeeklyTeamReport, weeklyRequest("GET", "/team", nil, other)).Want(200).JSON(&memberView)
	if len(memberView.Draft) > 0 || memberView.CanManage {
		t.Fatal("team draft leaked to a member")
	}
	testutil.Call(t, testHandler.SaveWeeklyReport, weeklyRequest("PUT", "/weekly", map[string]any{"content": weeklyContent("Published task"), "expected_revision": 0}, other)).Want(200)
	testutil.Call(t, testHandler.SubmitWeeklyReport, weeklyRequest("POST", "/weekly/submit", map[string]any{"expected_revision": 1}, other)).Want(200)
	testutil.Call(t, testHandler.GenerateWeeklyTeamReport, weeklyRequest("POST", "/team/generate", map[string]any{"expected_revision": team.Revision}, testUserID)).Want(200).JSON(&team)
	var snapshot wr.Snapshot
	if err := json.Unmarshal(team.Snapshot, &snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Collection.Reports) != 1 || snapshot.Collection.Reports[0].Content.Tasks[0].Title != "Published task" {
		t.Fatal("generation omitted submitted content")
	}
	var mine WeeklyReportResponse
	testutil.Call(t, testHandler.SaveWeeklyReport, weeklyRequest("PUT", "/weekly", map[string]any{"content": weeklyContent("Updated task"), "expected_revision": 2}, other)).Want(200).JSON(&mine)
	testutil.Call(t, testHandler.SubmitWeeklyReport, weeklyRequest("POST", "/weekly/submit", map[string]any{"expected_revision": mine.Revision}, other)).Want(200)
	testutil.Call(t, testHandler.GetWeeklyTeamReport, weeklyRequest("GET", "/team", nil, other)).Want(200).JSON(&memberView)
	if err := json.Unmarshal(memberView.Snapshot, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Collection.Reports[0].Content.Tasks[0].Title != "Published task" || memberView.Collection.Reports[0].Content.Tasks[0].Title != "Updated task" {
		t.Fatal("generated snapshot changed with later submissions")
	}
	testutil.Call(t, testHandler.GenerateWeeklyTeamReport, weeklyRequest("POST", "/team/generate", map[string]any{"expected_revision": team.Revision - 1}, testUserID)).Want(409)
	testutil.Call(t, testHandler.GenerateWeeklyTeamReport, weeklyRequest("POST", "/team/generate", map[string]any{"expected_revision": team.Revision}, testUserID)).Want(200).JSON(&team)
	if err := json.Unmarshal(team.Snapshot, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Collection.Reports[0].Content.Tasks[0].Title != "Updated task" {
		t.Fatal("regeneration did not refresh content")
	}
	// Published history and the original roster survive departure.
	dbfx.Exec(t, "DELETE FROM member WHERE workspace_id=$1 AND user_id=$2", testWorkspaceID, other)
	var week WeeklyReportWeekResponse
	testutil.Call(t, testHandler.ListWeeklyReports, weeklyRequest("GET", "/weekly?week=2026-11-09", nil, testUserID)).Want(http.StatusOK).JSON(&week)
	if len(week.Roster) != 2 || len(week.Reports) != 1 {
		t.Fatal("historical roster or report disappeared with membership")
	}
}

func TestWeeklyReportWorkspaceIsolation(t *testing.T) {
	weeklyCleanup(t)
	foreign := dbfx.Insert(t, "workspace", testutil.Cols{"name": "Other workspace", "slug": "weekly-other-" + testUserID[:8]})
	req := weeklyRequest("PUT", "/weekly", map[string]any{"content": weeklyContent("Foreign"), "expected_revision": 0}, testUserID)
	req.Header.Set("X-Workspace-ID", foreign)
	testutil.Call(t, testHandler.SaveWeeklyReport, req).Want(404)
	var n int
	if err := testPool.QueryRow(context.Background(), "SELECT count(*) FROM weekly_report WHERE workspace_id=$1", foreign).Scan(&n); err != nil || n != 0 {
		t.Fatalf("foreign write: count=%d err=%v", n, err)
	}
	req = testutil.WithURLParams(weeklyRequest("GET", "/weekly", nil, testUserID), "week", "2026-11-10")
	testutil.Call(t, testHandler.ListWeeklyReports, req).Want(400)
}

func TestWeeklyReportAuthenticatedAgentToken(t *testing.T) {
	weeklyCleanup(t)
	runtime := dbfx.Runtime(t, "Weekly runtime")
	agent := dbfx.Agent(t, "Weekly reporter", runtime)
	task := dbfx.Task(t, agent, testutil.Cols{"runtime_id": runtime})
	token, err := auth.GenerateAgentTaskToken()
	if err != nil {
		t.Fatal(err)
	}
	dbfx.Insert(t, "task_token", testutil.Cols{
		"token_hash": auth.HashToken(token), "task_id": task, "agent_id": agent,
		"workspace_id": testWorkspaceID, "user_id": testUserID,
		"expires_at": testutil.Raw("now() + interval '1 hour'"),
	})
	authenticate := func(handler http.HandlerFunc) http.HandlerFunc {
		return middleware.Auth(testHandler.Queries, nil, nil, nil)(handler).ServeHTTP
	}
	request := weeklyRequest("PUT", "/api/weekly-reports/2026-11-09/mine", map[string]any{"content": weeklyContent("Authenticated agent edit"), "expected_revision": 0}, "forged-user")
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-Agent-ID", testUserID)
	var report WeeklyReportResponse
	testutil.Call(t, authenticate(testHandler.SaveWeeklyReport), request).Want(200).JSON(&report)
	if report.UserID != testUserID || report.UpdatedByID != agent || report.UpdatedByType != "agent" {
		t.Fatalf("agent credential did not bind author and provenance: %+v", report)
	}
	request = weeklyRequest("GET", "/api/weekly-reports/2026-11-09/mine", nil, "forged-user")
	request.Header.Set("Authorization", "Bearer "+token)
	testutil.Call(t, authenticate(testHandler.GetWeeklyReport), request).Want(200).JSON(&report)
	if len(report.Draft) == 0 {
		t.Fatal("bound agent could not query its member's private draft")
	}
	request = weeklyRequest("PUT", "/api/weekly-team-reports/2026-11-09", map[string]any{"overview": wr.Overview{OnsiteRows: []wr.Progress{{Title: "Agent onsite update"}}}, "expected_revision": 0}, "forged-user")
	request.Header.Set("Authorization", "Bearer "+token)
	testutil.Call(t, authenticate(testHandler.SaveWeeklyTeamReport), request).Want(200)
	foreign := dbfx.Insert(t, "workspace", testutil.Cols{"name": "Foreign", "slug": "weekly-agent-" + agent[:8]})
	request = weeklyRequest("GET", "/api/weekly-reports/2026-11-09/mine", nil, testUserID)
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-Workspace-ID", foreign)
	testutil.Call(t, authenticate(testHandler.GetWeeklyReport), request).Want(403)
}

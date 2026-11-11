package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	wr "github.com/multica-ai/multica/server/pkg/weeklyreport"
)

type WeeklyReportResponse struct {
	ID            string          `json:"id"`
	UserID        string          `json:"user_id"`
	AuthorName    string          `json:"author_name"`
	WeekStart     string          `json:"week_start"`
	Revision      int32           `json:"revision"`
	Draft         json.RawMessage `json:"draft,omitempty"`
	Published     json.RawMessage `json:"published,omitempty"`
	PublishedAt   *string         `json:"published_at"`
	UpdatedAt     string          `json:"updated_at"`
	UpdatedByType string          `json:"updated_by_type"`
	UpdatedByID   string          `json:"updated_by_id"`
}

type WeeklyReportWeekResponse struct {
	WeekStart string                 `json:"week_start"`
	Roster    []wr.Member            `json:"roster"`
	Reports   []WeeklyReportResponse `json:"reports"`
}

type WeeklyTeamReportResponse struct {
	ID          string          `json:"id"`
	WeekStart   string          `json:"week_start"`
	Revision    int32           `json:"revision"`
	Draft       json.RawMessage `json:"draft,omitempty"`
	Snapshot    json.RawMessage `json:"snapshot,omitempty"`
	GeneratedAt *string         `json:"generated_at"`
	CanManage   bool            `json:"can_manage"`
	Collection  wr.Collection   `json:"collection"`
}

type weeklyScope struct {
	workspace pgtype.UUID
	user      pgtype.UUID
	week      pgtype.Date
	member    db.Member
}

func (h *Handler) weeklyScope(w http.ResponseWriter, r *http.Request, withWeek bool) (weeklyScope, bool) {
	ws := h.resolveWorkspaceID(r)
	member, ok := h.workspaceMember(w, r, ws)
	if !ok {
		return weeklyScope{}, false
	}
	scope := weeklyScope{workspace: member.WorkspaceID, user: member.UserID, member: member}
	if withWeek {
		week := chi.URLParam(r, "week")
		if week == "" {
			week = r.URL.Query().Get("week")
		}
		day, err := wr.Week(week)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return weeklyScope{}, false
		}
		scope.week = pgtype.Date{Time: day, Valid: true}
	}
	return scope, true
}

func weeklyDecode(w http.ResponseWriter, r *http.Request, out any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		writeError(w, http.StatusBadRequest, "invalid weekly report request")
		return false
	}
	if decoder.Decode(new(any)) != io.EOF {
		writeError(w, http.StatusBadRequest, "expected a single JSON document")
		return false
	}
	return true
}

func weeklyRevision(w http.ResponseWriter, revision *int32) bool {
	if revision == nil || *revision < 0 {
		writeError(w, http.StatusBadRequest, "expected_revision is required and must be non-negative")
		return false
	}
	return true
}

func weeklyWriteError(w http.ResponseWriter, err error) {
	if errors.Is(err, pgx.ErrNoRows) {
		writeErrorCode(w, http.StatusConflict, "revision_conflict", "report changed; reload it before saving again")
		return
	}
	writeError(w, http.StatusInternalServerError, "failed to save weekly report")
}

func weeklyPersonalResponse(row db.WeeklyReport, viewer pgtype.UUID) WeeklyReportResponse {
	result := WeeklyReportResponse{
		ID: uuidToString(row.ID), UserID: uuidToString(row.UserID), AuthorName: row.AuthorName,
		WeekStart: row.WeekStart.Time.Format(time.DateOnly), Revision: row.Revision,
		Published: row.Published, PublishedAt: timestampToPtr(row.PublishedAt),
		UpdatedAt: timestampToString(row.UpdatedAt), UpdatedByType: row.UpdatedByType, UpdatedByID: uuidToString(row.UpdatedByID),
	}
	if row.UserID == viewer {
		result.Draft = row.Draft
	}
	return result
}

// The first access fixes the week's roster. Published reports retain their author
// even after membership changes, and later participants are included by their row.
func (h *Handler) weeklyRoster(ctx context.Context, q *db.Queries, scope weeklyScope) ([]wr.Member, error) {
	members, err := q.ListMembersWithUser(ctx, scope.workspace)
	if err != nil {
		return nil, err
	}
	roster := make([]wr.Member, 0, len(members))
	for _, member := range members {
		roster = append(roster, wr.Member{UserID: uuidToString(member.UserID), Name: member.UserName, AvatarURL: member.UserAvatarUrl.String})
	}
	encoded, err := json.Marshal(roster)
	if err != nil {
		return nil, err
	}
	period, err := q.EnsureWeeklyReportPeriod(ctx, db.EnsureWeeklyReportPeriodParams{WorkspaceID: scope.workspace, WeekStart: scope.week, Roster: encoded})
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(period.Roster, &roster)
	return roster, err
}

func (h *Handler) weeklyCollection(ctx context.Context, q *db.Queries, scope weeklyScope) (wr.Collection, []db.WeeklyReport, error) {
	roster, err := h.weeklyRoster(ctx, q, scope)
	if err != nil {
		return wr.Collection{}, nil, err
	}
	rows, err := q.ListWeeklyReports(ctx, db.ListWeeklyReportsParams{WorkspaceID: scope.workspace, WeekStart: scope.week})
	if err != nil {
		return wr.Collection{}, nil, err
	}
	result := wr.Collection{Roster: roster, Reports: []wr.Contribution{}, Missing: []wr.Member{}}
	known, published := map[string]wr.Member{}, map[string]bool{}
	for _, member := range roster {
		known[member.UserID] = member
	}
	for _, row := range rows {
		if len(row.Published) == 0 {
			continue
		}
		id := uuidToString(row.UserID)
		member, exists := known[id]
		if !exists {
			member = wr.Member{UserID: id, Name: row.AuthorName}
			result.Roster = append(result.Roster, member)
			known[id] = member
		}
		var content wr.Content
		if err := json.Unmarshal(row.Published, &content); err != nil {
			return wr.Collection{}, nil, err
		}
		result.Reports = append(result.Reports, wr.Contribution{Member: member, Content: content, PublishedAt: timestampToString(row.PublishedAt)})
		published[id] = true
	}
	for _, member := range result.Roster {
		if !published[member.UserID] {
			result.Missing = append(result.Missing, member)
		}
	}
	return result, rows, nil
}

func (h *Handler) ListWeeklyReports(w http.ResponseWriter, r *http.Request) {
	scope, ok := h.weeklyScope(w, r, true)
	if !ok {
		return
	}
	collection, rows, err := h.weeklyCollection(r.Context(), h.Queries, scope)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list weekly reports")
		return
	}
	result := WeeklyReportWeekResponse{WeekStart: scope.week.Time.Format(time.DateOnly), Roster: collection.Roster, Reports: []WeeklyReportResponse{}}
	for _, row := range rows {
		if row.UserID == scope.user || len(row.Published) > 0 {
			result.Reports = append(result.Reports, weeklyPersonalResponse(row, scope.user))
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) GetWeeklyReport(w http.ResponseWriter, r *http.Request) {
	scope, ok := h.weeklyScope(w, r, true)
	if !ok {
		return
	}
	user := scope.user
	if target := chi.URLParam(r, "userId"); target != "" {
		var valid bool
		user, valid = parseUUIDOrBadRequest(w, target, "user_id")
		if !valid {
			return
		}
	}
	row, err := h.Queries.GetWeeklyReport(r.Context(), db.GetWeeklyReportParams{WorkspaceID: scope.workspace, WeekStart: scope.week, UserID: user})
	if errors.Is(err, pgx.ErrNoRows) && user == scope.user {
		writeJSON(w, http.StatusOK, WeeklyReportResponse{UserID: uuidToString(user), WeekStart: scope.week.Time.Format(time.DateOnly), Draft: json.RawMessage(`{"tasks":[],"plan":""}`)})
		return
	}
	if errors.Is(err, pgx.ErrNoRows) || err == nil && user != scope.user && len(row.Published) == 0 {
		writeError(w, http.StatusNotFound, "weekly report not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read weekly report")
		return
	}
	writeJSON(w, http.StatusOK, weeklyPersonalResponse(row, scope.user))
}

func (h *Handler) WeeklyReportHistory(w http.ResponseWriter, r *http.Request) {
	scope, ok := h.weeklyScope(w, r, false)
	if !ok {
		return
	}
	user := scope.user
	if value := r.URL.Query().Get("user_id"); value != "" {
		var valid bool
		user, valid = parseUUIDOrBadRequest(w, value, "user_id")
		if !valid {
			return
		}
	}
	before := time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC)
	if value := r.URL.Query().Get("before"); value != "" {
		var err error
		before, err = time.Parse(time.DateOnly, value)
		if err != nil {
			writeError(w, http.StatusBadRequest, "before must be YYYY-MM-DD")
			return
		}
	}
	limit := 52
	if value := r.URL.Query().Get("limit"); value != "" {
		var err error
		limit, err = strconv.Atoi(value)
		if err != nil || limit < 1 || limit > 100 {
			writeError(w, http.StatusBadRequest, "limit must be between 1 and 100")
			return
		}
	}
	rows, err := h.Queries.ListWeeklyReportHistory(r.Context(), db.ListWeeklyReportHistoryParams{WorkspaceID: scope.workspace, UserID: user, ViewerID: scope.user, BeforeWeek: pgtype.Date{Time: before, Valid: true}, Limit: int32(limit)})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read weekly report history")
		return
	}
	result := make([]WeeklyReportResponse, 0, len(rows))
	for _, row := range rows {
		result = append(result, weeklyPersonalResponse(row, scope.user))
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) SaveWeeklyReport(w http.ResponseWriter, r *http.Request) {
	scope, ok := h.weeklyScope(w, r, true)
	if !ok {
		return
	}
	var req struct {
		Content          *wr.Content `json:"content"`
		ExpectedRevision *int32      `json:"expected_revision"`
	}
	if !weeklyDecode(w, r, &req) || !weeklyRevision(w, req.ExpectedRevision) {
		return
	}
	if req.Content == nil {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}
	if err := req.Content.Normalize(false); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := h.weeklyRoster(r.Context(), h.Queries, scope); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to initialize weekly roster")
		return
	}
	actorType, actorID := h.resolveActor(r, uuidToString(scope.user), uuidToString(scope.workspace))
	actor, ok := parseUUIDOrBadRequest(w, actorID, "actor_id")
	if !ok {
		return
	}
	body, _ := json.Marshal(req.Content)
	var row db.WeeklyReport
	var err error
	if *req.ExpectedRevision == 0 {
		user, loadErr := h.Queries.GetUser(r.Context(), scope.user)
		if loadErr != nil {
			writeError(w, http.StatusNotFound, "member not found")
			return
		}
		row, err = h.Queries.SaveWeeklyReportDraft(r.Context(), db.SaveWeeklyReportDraftParams{WorkspaceID: scope.workspace, UserID: scope.user, WeekStart: scope.week, AuthorName: user.Name, Draft: body, UpdatedByType: actorType, UpdatedByID: actor, ExpectedRevision: 0})
	} else {
		row, err = h.Queries.UpdateWeeklyReportDraft(r.Context(), db.UpdateWeeklyReportDraftParams{WorkspaceID: scope.workspace, UserID: scope.user, WeekStart: scope.week, Draft: body, UpdatedByType: actorType, UpdatedByID: actor, Revision: *req.ExpectedRevision})
	}
	if err != nil {
		weeklyWriteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, weeklyPersonalResponse(row, scope.user))
}

func (h *Handler) SubmitWeeklyReport(w http.ResponseWriter, r *http.Request) {
	scope, ok := h.weeklyScope(w, r, true)
	if !ok {
		return
	}
	var req struct {
		ExpectedRevision *int32 `json:"expected_revision"`
	}
	if !weeklyDecode(w, r, &req) || !weeklyRevision(w, req.ExpectedRevision) {
		return
	}
	row, err := h.Queries.GetWeeklyReport(r.Context(), db.GetWeeklyReportParams{WorkspaceID: scope.workspace, WeekStart: scope.week, UserID: scope.user})
	if err != nil {
		weeklyWriteError(w, err)
		return
	}
	var content wr.Content
	if err := json.Unmarshal(row.Draft, &content); err != nil {
		writeError(w, http.StatusInternalServerError, "invalid stored weekly report")
		return
	}
	if err := content.Normalize(true); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	actorType, actorID := h.resolveActor(r, uuidToString(scope.user), uuidToString(scope.workspace))
	actor, ok := parseUUIDOrBadRequest(w, actorID, "actor_id")
	if !ok {
		return
	}
	row, err = h.Queries.PublishWeeklyReport(r.Context(), db.PublishWeeklyReportParams{WorkspaceID: scope.workspace, WeekStart: scope.week, UserID: scope.user, UpdatedByType: actorType, UpdatedByID: actor, Revision: *req.ExpectedRevision})
	if err != nil {
		weeklyWriteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, weeklyPersonalResponse(row, scope.user))
}

func (h *Handler) weeklyTeamResponse(ctx context.Context, scope weeklyScope, row db.WeeklyTeamReport) (WeeklyTeamReportResponse, error) {
	collection, _, err := h.weeklyCollection(ctx, h.Queries, scope)
	result := WeeklyTeamReportResponse{ID: uuidToString(row.ID), WeekStart: scope.week.Time.Format(time.DateOnly), Revision: row.Revision, Snapshot: row.Snapshot, GeneratedAt: timestampToPtr(row.GeneratedAt), CanManage: roleAllowed(scope.member.Role, "owner", "admin"), Collection: collection}
	if result.CanManage {
		result.Draft = row.Draft
		if len(result.Draft) == 0 {
			result.Draft = json.RawMessage(`{"version_name":"","version_rows":[],"onsite_rows":[]}`)
		}
	}
	return result, err
}

func (h *Handler) GetWeeklyTeamReport(w http.ResponseWriter, r *http.Request) {
	scope, ok := h.weeklyScope(w, r, true)
	if !ok {
		return
	}
	row, err := h.Queries.GetWeeklyTeamReport(r.Context(), db.GetWeeklyTeamReportParams{WorkspaceID: scope.workspace, WeekStart: scope.week})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, "failed to read team report")
		return
	}
	result, err := h.weeklyTeamResponse(r.Context(), scope, row)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to collect weekly reports")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) SaveWeeklyTeamReport(w http.ResponseWriter, r *http.Request) {
	scope, ok := h.weeklyScope(w, r, true)
	if !ok {
		return
	}
	if !roleAllowed(scope.member.Role, "owner", "admin") {
		writeError(w, http.StatusForbidden, "team reports require owner or admin")
		return
	}
	var req struct {
		Overview         *wr.Overview `json:"overview"`
		ExpectedRevision *int32       `json:"expected_revision"`
	}
	if !weeklyDecode(w, r, &req) || !weeklyRevision(w, req.ExpectedRevision) {
		return
	}
	if req.Overview == nil {
		writeError(w, http.StatusBadRequest, "overview is required")
		return
	}
	if err := req.Overview.Normalize(false); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	actorType, actorID := h.resolveActor(r, uuidToString(scope.user), uuidToString(scope.workspace))
	actor, ok := parseUUIDOrBadRequest(w, actorID, "actor_id")
	if !ok {
		return
	}
	body, _ := json.Marshal(req.Overview)
	var row db.WeeklyTeamReport
	var err error
	if *req.ExpectedRevision == 0 {
		row, err = h.Queries.CreateWeeklyTeamReport(r.Context(), db.CreateWeeklyTeamReportParams{WorkspaceID: scope.workspace, WeekStart: scope.week, Draft: body, UpdatedByType: actorType, UpdatedByID: actor})
	} else {
		row, err = h.Queries.UpdateWeeklyTeamReport(r.Context(), db.UpdateWeeklyTeamReportParams{WorkspaceID: scope.workspace, WeekStart: scope.week, Draft: body, UpdatedByType: actorType, UpdatedByID: actor, Revision: *req.ExpectedRevision})
	}
	if err != nil {
		weeklyWriteError(w, err)
		return
	}
	result, err := h.weeklyTeamResponse(r.Context(), scope, row)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to collect weekly reports")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) GenerateWeeklyTeamReport(w http.ResponseWriter, r *http.Request) {
	scope, ok := h.weeklyScope(w, r, true)
	if !ok {
		return
	}
	if !roleAllowed(scope.member.Role, "owner", "admin") {
		writeError(w, http.StatusForbidden, "team reports require owner or admin")
		return
	}
	var req struct {
		ExpectedRevision *int32 `json:"expected_revision"`
	}
	if !weeklyDecode(w, r, &req) || !weeklyRevision(w, req.ExpectedRevision) {
		return
	}
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to begin team report generation")
		return
	}
	defer tx.Rollback(r.Context())
	q := h.Queries.WithTx(tx)
	row, err := q.GetWeeklyTeamReport(r.Context(), db.GetWeeklyTeamReportParams{WorkspaceID: scope.workspace, WeekStart: scope.week})
	if err != nil {
		weeklyWriteError(w, err)
		return
	}
	var overview wr.Overview
	if err := json.Unmarshal(row.Draft, &overview); err != nil {
		writeError(w, http.StatusInternalServerError, "invalid stored overview")
		return
	}
	if err := overview.Normalize(true); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	collection, _, err := h.weeklyCollection(r.Context(), q, scope)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to collect submitted weekly reports")
		return
	}
	body, _ := json.Marshal(wr.Snapshot{Overview: overview, Collection: collection})
	actorType, actorID := h.resolveActor(r, uuidToString(scope.user), uuidToString(scope.workspace))
	actor, ok := parseUUIDOrBadRequest(w, actorID, "actor_id")
	if !ok {
		return
	}
	row, err = q.GenerateWeeklyTeamReport(r.Context(), db.GenerateWeeklyTeamReportParams{WorkspaceID: scope.workspace, WeekStart: scope.week, Snapshot: body, UpdatedByType: actorType, UpdatedByID: actor, Revision: *req.ExpectedRevision})
	if err != nil {
		weeklyWriteError(w, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate team report")
		return
	}
	result, err := h.weeklyTeamResponse(r.Context(), scope, row)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read generated report")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

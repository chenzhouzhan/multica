-- name: EnsureWeeklyReportPeriod :one
INSERT INTO weekly_report_period (workspace_id, week_start, roster)
VALUES ($1, $2, $3)
ON CONFLICT (workspace_id, week_start) DO UPDATE SET roster = weekly_report_period.roster
RETURNING *;

-- name: GetWeeklyReport :one
SELECT * FROM weekly_report WHERE workspace_id = $1 AND week_start = $2 AND user_id = $3;

-- name: ListWeeklyReports :many
SELECT * FROM weekly_report WHERE workspace_id = $1 AND week_start = $2 ORDER BY created_at, id;

-- name: ListWeeklyReportHistory :many
SELECT * FROM weekly_report
WHERE workspace_id = $1 AND user_id = $2
  AND week_start < sqlc.arg(before_week)::date
  AND (user_id = sqlc.arg(viewer_id)::uuid OR published IS NOT NULL)
ORDER BY week_start DESC LIMIT $3;

-- name: SaveWeeklyReportDraft :one
INSERT INTO weekly_report (workspace_id, user_id, week_start, author_name, draft, updated_by_type, updated_by_id)
SELECT $1, $2, $3, $4, $5, $6, $7 WHERE sqlc.arg(expected_revision)::integer = 0
ON CONFLICT (workspace_id, user_id, week_start) DO UPDATE SET
    draft = EXCLUDED.draft, revision = weekly_report.revision + 1,
    updated_by_type = EXCLUDED.updated_by_type, updated_by_id = EXCLUDED.updated_by_id, updated_at = now()
WHERE weekly_report.revision = sqlc.arg(expected_revision)
RETURNING *;

-- name: UpdateWeeklyReportDraft :one
UPDATE weekly_report SET draft = $4, revision = revision + 1,
    updated_by_type = $5, updated_by_id = $6, updated_at = now()
WHERE workspace_id = $1 AND week_start = $2 AND user_id = $3 AND revision = $7
RETURNING *;

-- name: PublishWeeklyReport :one
UPDATE weekly_report SET published = draft, published_at = now(), revision = revision + 1,
    updated_by_type = $4, updated_by_id = $5, updated_at = now()
WHERE workspace_id = $1 AND week_start = $2 AND user_id = $3 AND revision = $6
RETURNING *;

-- name: GetWeeklyTeamReport :one
SELECT * FROM weekly_team_report WHERE workspace_id = $1 AND week_start = $2;

-- name: CreateWeeklyTeamReport :one
INSERT INTO weekly_team_report (workspace_id, week_start, draft, updated_by_type, updated_by_id)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (workspace_id, week_start) DO NOTHING
RETURNING *;

-- name: UpdateWeeklyTeamReport :one
UPDATE weekly_team_report SET draft = $3, revision = revision + 1,
    updated_by_type = $4, updated_by_id = $5, updated_at = now()
WHERE workspace_id = $1 AND week_start = $2 AND revision = $6
RETURNING *;

-- name: GenerateWeeklyTeamReport :one
UPDATE weekly_team_report SET snapshot = $3, generated_at = now(), revision = revision + 1,
    updated_by_type = $4, updated_by_id = $5, updated_at = now()
WHERE workspace_id = $1 AND week_start = $2 AND revision = $6
RETURNING *;

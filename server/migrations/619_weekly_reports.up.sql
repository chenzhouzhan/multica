CREATE TABLE weekly_report_period (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    week_start DATE NOT NULL CHECK (extract(isodow FROM week_start) = 1),
    roster JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_weekly_report_period_workspace_week ON weekly_report_period (workspace_id, week_start);

CREATE TABLE weekly_report (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    user_id UUID NOT NULL,
    week_start DATE NOT NULL CHECK (extract(isodow FROM week_start) = 1),
    author_name TEXT NOT NULL,
    draft JSONB NOT NULL DEFAULT '{"tasks":[],"plan":""}',
    published JSONB,
    revision INTEGER NOT NULL DEFAULT 1,
    published_at TIMESTAMPTZ,
    updated_by_type TEXT NOT NULL,
    updated_by_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_weekly_report_workspace_member_week ON weekly_report (workspace_id, user_id, week_start);
CREATE INDEX idx_weekly_report_workspace_week ON weekly_report (workspace_id, week_start);

CREATE TABLE weekly_team_report (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    week_start DATE NOT NULL CHECK (extract(isodow FROM week_start) = 1),
    draft JSONB NOT NULL DEFAULT '{"version_name":"","version_rows":[],"onsite_rows":[]}',
    snapshot JSONB,
    revision INTEGER NOT NULL DEFAULT 1,
    generated_at TIMESTAMPTZ,
    updated_by_type TEXT NOT NULL,
    updated_by_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_weekly_team_report_workspace_week ON weekly_team_report (workspace_id, week_start);

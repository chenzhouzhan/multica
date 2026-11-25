CREATE TABLE IF NOT EXISTS infrastructure_connection (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    name TEXT NOT NULL,
    base_url TEXT NOT NULL,
    username TEXT NOT NULL,
    password_ciphertext BYTEA NOT NULL,
    ca_pem TEXT NOT NULL DEFAULT '',
    restart_grants JSONB NOT NULL DEFAULT '[]',
    created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    skip_tls_verify BOOLEAN NOT NULL DEFAULT false
);

CREATE INDEX IF NOT EXISTS infrastructure_connection_workspace_idx ON infrastructure_connection (workspace_id, created_at);

CREATE TABLE IF NOT EXISTS infrastructure_operation (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    connection_id UUID NOT NULL,
    cluster_id BIGINT NOT NULL,
    service_id BIGINT NOT NULL,
    role_id BIGINT NOT NULL,
    actor_type TEXT NOT NULL,
    actor_id UUID NOT NULL,
    reason TEXT NOT NULL,
    upstream_job_id BIGINT,
    status TEXT NOT NULL DEFAULT 'submitting',
    error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS infrastructure_operation_workspace_idx ON infrastructure_operation (workspace_id, connection_id, created_at DESC);

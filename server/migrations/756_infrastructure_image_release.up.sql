ALTER TABLE infrastructure_connection ADD COLUMN IF NOT EXISTS execution_access_ciphertext BYTEA;
ALTER TABLE infrastructure_operation ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'restart_role';
ALTER TABLE infrastructure_operation ADD COLUMN IF NOT EXISTS details JSONB NOT NULL DEFAULT '{}';

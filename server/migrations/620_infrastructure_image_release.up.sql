ALTER TABLE infrastructure_connection ADD COLUMN execution_access_ciphertext BYTEA;
ALTER TABLE infrastructure_operation ADD COLUMN kind TEXT NOT NULL DEFAULT 'restart_role';
ALTER TABLE infrastructure_operation ADD COLUMN details JSONB NOT NULL DEFAULT '{}';

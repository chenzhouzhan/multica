-- name: ListInfrastructureConnections :many
SELECT * FROM infrastructure_connection WHERE workspace_id = $1 ORDER BY created_at;

-- name: GetInfrastructureConnection :one
SELECT * FROM infrastructure_connection WHERE id = $1 AND workspace_id = $2;

-- name: CreateInfrastructureConnection :one
INSERT INTO infrastructure_connection (workspace_id, name, base_url, username, password_ciphertext, ca_pem, created_by, skip_tls_verify)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING *;

-- name: UpdateInfrastructureConnection :one
UPDATE infrastructure_connection SET name = $3, base_url = $4, username = $5, password_ciphertext = $6, ca_pem = $7, skip_tls_verify = $8, restart_grants = '[]', updated_at = now()
WHERE id = $1 AND workspace_id = $2 RETURNING *;

-- name: UpdateInfrastructureGrants :one
UPDATE infrastructure_connection SET restart_grants = $3, updated_at = now()
WHERE id = $1 AND workspace_id = $2 RETURNING *;

-- name: DeleteInfrastructureOperations :exec
DELETE FROM infrastructure_operation WHERE connection_id = $1 AND workspace_id = $2;

-- name: DeleteInfrastructureConnection :exec
DELETE FROM infrastructure_connection WHERE id = $1 AND workspace_id = $2;

-- name: CreateInfrastructureOperation :one
INSERT INTO infrastructure_operation (workspace_id, connection_id, cluster_id, service_id, role_id, actor_type, actor_id, reason)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING *;

-- name: UpdateInfrastructureOperation :one
UPDATE infrastructure_operation SET upstream_job_id = $3, status = $4, error = $5, updated_at = now()
WHERE id = $1 AND workspace_id = $2 RETURNING *;

-- name: ListInfrastructureOperations :many
SELECT * FROM infrastructure_operation WHERE workspace_id = $1 AND connection_id = $2 ORDER BY created_at DESC LIMIT 50;

-- name: GetInfrastructureOperation :one
SELECT * FROM infrastructure_operation WHERE id = $1 AND workspace_id = $2 AND connection_id = $3;

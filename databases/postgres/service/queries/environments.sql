-- name: GetEnvironments :many
SELECT * FROM control_plane.environments;

-- name: GetEnvironmentByUUID :one
SELECT * FROM control_plane.environments WHERE uuid = $1;

-- name: GetEnvironmentsByTenantUUID :many
SELECT e.* FROM control_plane.environments e
INNER JOIN control_plane.tenants_environments te ON e.uuid = te.environment_uuid
WHERE te.tenant_uuid = $1;

-- name: CreateEnvironment :exec
INSERT INTO control_plane.environments (uuid, name, config)
VALUES ($1, $2, $3);

-- name: DeleteEnvironment :exec
DELETE FROM control_plane.environments WHERE uuid = $1;
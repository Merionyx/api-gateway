-- name: GetEnvironments :many
SELECT * FROM control_plane.environments
ORDER BY name;

-- name: GetEnvironmentByUUID :one
SELECT * FROM control_plane.environments WHERE uuid = $1;

-- name: GetEnvironmentByName :one
SELECT * FROM control_plane.environments WHERE name = $1;

-- name: GetEnvironmentsByTenantUUID :many
SELECT e.* FROM control_plane.environments e
INNER JOIN control_plane.tenants_environments te ON e.uuid = te.environment_uuid
WHERE te.tenant_uuid = $1
ORDER BY e.name;

-- name: CreateEnvironment :exec
INSERT INTO control_plane.environments (uuid, name, config)
VALUES ($1, $2, $3);

-- name: UpdateEnvironment :exec
UPDATE control_plane.environments SET name = $2, config = $3 WHERE uuid = $1;

-- name: MapTenantToEnvironment :exec
INSERT INTO control_plane.tenants_environments (tenant_uuid, environment_uuid)
VALUES ($1, $2);

-- name: UnmapTenantFromEnvironment :exec
DELETE FROM control_plane.tenants_environments 
WHERE tenant_uuid = $1 AND environment_uuid = $2;

-- name: DeleteEnvironment :exec
DELETE FROM control_plane.environments WHERE uuid = $1;

-- name: DeleteEnvironmentTenantMappings :exec
DELETE FROM control_plane.tenants_environments WHERE environment_uuid = $1;

-- name: DeleteEnvironmentListenerMappings :exec
DELETE FROM control_plane.listeners_environments WHERE environment_uuid = $1;
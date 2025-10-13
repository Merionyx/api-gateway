-- name: GetTenants :many
SELECT * FROM control_plane.tenants
ORDER BY name;

-- name: GetTenantByUUID :one
SELECT * FROM control_plane.tenants WHERE uuid = $1;

-- name: GetTenantByName :one
SELECT * FROM control_plane.tenants WHERE name = $1;

-- name: CreateTenant :exec
INSERT INTO control_plane.tenants (uuid, name)
VALUES ($1, $2);

-- name: UpdateTenant :exec
UPDATE control_plane.tenants SET name = $2 WHERE uuid = $1;

-- name: DeleteTenant :exec
DELETE FROM control_plane.tenants WHERE uuid = $1;

-- name: DeleteTenantEnvironmentMappings :exec
DELETE FROM control_plane.tenants_environments WHERE tenant_uuid = $1;
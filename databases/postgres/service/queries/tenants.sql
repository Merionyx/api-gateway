-- name: GetTenants :many
SELECT * FROM control_plane.tenants;

-- name: GetTenantByUUID :one
SELECT * FROM control_plane.tenants WHERE uuid = $1;

-- name: CreateTenant :exec
INSERT INTO control_plane.tenants (uuid, name)
VALUES ($1, $2);

-- name: DeleteTenant :exec
DELETE FROM control_plane.tenants WHERE uuid = $1;
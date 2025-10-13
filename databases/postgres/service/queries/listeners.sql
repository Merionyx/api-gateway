-- name: GetListeners :many
SELECT * FROM control_plane.listeners;

-- name: GetListenerByUUID :one
SELECT * FROM control_plane.listeners WHERE uuid = $1;

-- name: GetListenersByEnvironmentUUID :many
SELECT l.* FROM control_plane.listeners l
INNER JOIN control_plane.listeners_environments le ON l.uuid = le.listener_uuid
WHERE le.environment_uuid = $1;

-- name: CreateListener :exec
INSERT INTO control_plane.listeners (uuid, name, config)
VALUES ($1, $2, $3);

-- name: UpdateListener :exec
UPDATE control_plane.listeners SET name = $2, config = $3 WHERE uuid = $1;

-- name: DeleteListener :exec
DELETE FROM control_plane.listeners WHERE uuid = $1;
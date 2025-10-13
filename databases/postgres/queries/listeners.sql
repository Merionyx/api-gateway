-- name: GetListeners :many
SELECT * FROM control_plane.listeners
ORDER BY name;

-- name: GetListenerByUUID :one
SELECT * FROM control_plane.listeners WHERE uuid = $1;

-- name: GetListenerByName :one
SELECT * FROM control_plane.listeners WHERE name = $1;

-- name: GetListenersByEnvironmentUUID :many
SELECT l.* FROM control_plane.listeners l
INNER JOIN control_plane.listeners_environments le ON l.uuid = le.listener_uuid
WHERE le.environment_uuid = $1
ORDER BY l.name;

-- name: CreateListener :exec
INSERT INTO control_plane.listeners (uuid, name, config)
VALUES ($1, $2, $3);

-- name: UpdateListener :exec
UPDATE control_plane.listeners SET name = $2, config = $3 WHERE uuid = $1;

-- name: MapListenerToEnvironment :exec
INSERT INTO control_plane.listeners_environments (listener_uuid, environment_uuid)
VALUES ($1, $2);

-- name: UnmapListenerFromEnvironment :exec
DELETE FROM control_plane.listeners_environments 
WHERE listener_uuid = $1 AND environment_uuid = $2;

-- name: DeleteListener :exec
DELETE FROM control_plane.listeners WHERE uuid = $1;

-- name: DeleteListenerEnvironmentMappings :exec
DELETE FROM control_plane.listeners_environments WHERE listener_uuid = $1;
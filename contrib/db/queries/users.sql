-- CreateUser registers a new user in the system.
-- Idempotent: does nothing if user already exists (ON CONFLICT DO NOTHING).
-- name: CreateUser :exec
INSERT INTO agents.users (id)
VALUES (sqlc.narg('id'))
ON CONFLICT (id) DO NOTHING;

-- GetUser validates if a user exists.
-- Used during authentication/authorization checks.
--
-- Returns: id if found, error if not found.
-- name: GetUser :one
SELECT id
FROM agents.users
WHERE id = sqlc.narg('user_id');

-- DeleteUser removes a user and all cascade-linked data (if configured).
-- Currently handles basic user record deletion.
-- name: DeleteUser :exec
DELETE FROM agents.users
WHERE id = sqlc.narg('id');

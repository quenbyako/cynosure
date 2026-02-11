-- CreateUser registers a new user in the system.
-- Idempotent: does nothing if user already exists (ON CONFLICT DO NOTHING).
-- name: CreateUser :exec
INSERT INTO agents.users (id)
VALUES (sqlc.arg('id'))
ON CONFLICT (id) DO NOTHING;

-- GetUser validates if a user exists.
-- Used during authentication/authorization checks.
--
-- Returns: id if found, error if not found.
-- name: GetUser :one
SELECT id
FROM agents.users
WHERE id = sqlc.arg('user_id');

-- DeleteUser removes a user and all cascade-linked data (if configured).
-- Currently handles basic user record deletion.
-- name: DeleteUser :exec
DELETE FROM agents.users
WHERE id = sqlc.arg('id');

-- LookupUserByTelegram returns the user ID associated with the given telegram ID.
-- name: LookupUserByTelegram :one
SELECT user_id
FROM agents.user_telegram
WHERE telegram_id = sqlc.arg('telegram_id');

-- CreateUserTelegram associates a user with a telegram ID.
-- name: CreateUserTelegram :exec
INSERT INTO agents.user_telegram (user_id, telegram_id)
VALUES (sqlc.arg('user_id'), sqlc.arg('telegram_id'));

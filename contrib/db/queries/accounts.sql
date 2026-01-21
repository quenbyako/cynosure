-- name: ListAccountIDs :many
SELECT id, server, user_id FROM agents.mcp_accounts
WHERE user_id = $1 AND deleted_at IS NULL;

-- name: GetAccount :one
SELECT * FROM agents.mcp_accounts
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetAccountsBatch :many
SELECT * FROM agents.mcp_accounts
WHERE id = ANY(@ids::uuid[]) AND deleted_at IS NULL;

-- name: UpsertAccount :exec
INSERT INTO agents.mcp_accounts (id, user_id, server, name, description, token, deleted_at)
VALUES ($1, $2, $3, $4, $5, $6, NULL)
ON CONFLICT (id) DO UPDATE
SET user_id = EXCLUDED.user_id,
    server = EXCLUDED.server,
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    token = EXCLUDED.token,
    deleted_at = NULL;

-- name: SoftDeleteAccount :exec
UPDATE agents.mcp_accounts
SET deleted_at = NOW()
WHERE id = $1;

-- name: DeleteAccountTools :exec
DELETE FROM agents.mcp_tools
WHERE account = $1;

-- name: InsertAccountTool :exec
INSERT INTO agents.mcp_tools (id, account, name, input, output)
VALUES ($1, $2, $3, $4, $5);

-- name: ListToolsForAccounts :many
SELECT * FROM agents.mcp_tools
WHERE account = ANY(@accounts::uuid[]);

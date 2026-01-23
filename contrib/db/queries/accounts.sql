-- name: ListAccountIDs :many
SELECT id, server, user_id FROM agents.mcp_accounts
WHERE user_id = $1 AND deleted_at IS NULL;

-- name: GetAccount :one
SELECT
	a.id, a.user_id, a.server, a.name, a.description, a.deleted_at,
	ot.type, ot.access_token, ot.refresh_token, ot.expiry
FROM agents.mcp_accounts a
LEFT JOIN agents.oauth_tokens ot ON a.id = ot.account_id
WHERE a.id = $1 AND a.deleted_at IS NULL;

-- name: GetAccountsBatch :many
SELECT
	a.id, a.user_id, a.server, a.name, a.description, a.deleted_at,
	ot.type, ot.access_token, ot.refresh_token, ot.expiry
FROM agents.mcp_accounts a
LEFT JOIN agents.oauth_tokens ot ON a.id = ot.account_id
WHERE a.id = ANY(@ids::uuid[]) AND a.deleted_at IS NULL;

-- name: UpsertAccount :exec
INSERT INTO agents.mcp_accounts (id, user_id, server, name, description, deleted_at)
VALUES ($1, $2, $3, $4, $5, NULL)
ON CONFLICT (id) DO UPDATE
SET user_id = EXCLUDED.user_id,
    server = EXCLUDED.server,
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    deleted_at = NULL;

-- name: AddOAuthToken :exec
INSERT INTO agents.oauth_tokens (account_id, type, access_token, refresh_token, expiry)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (account_id) DO UPDATE SET
	type = EXCLUDED.type,
	access_token = EXCLUDED.access_token,
	refresh_token = EXCLUDED.refresh_token,
	expiry = EXCLUDED.expiry;

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

-- name: DeleteAccountToken :exec
DELETE FROM agents.oauth_tokens
WHERE account_id = $1;

-- REMINDER: After modifying this file, regenerate Go code:
--   cd contrib/db && go generate ./...

-- name: ListAccountIDs :many
SELECT id, server_id, name
FROM agents.mcp_accounts
WHERE user_id = sqlc.arg('user_id')::UUID AND deleted_at IS NULL;

-- GetAccount retrieves a single MCP account by ID, including its OAuth token if present.
-- The deleted_at filter ensures soft-deleted accounts are excluded.
--
-- Returns: Account details and associated OAuth token info (null if no token).
-- name: GetAccount :one
SELECT
	a.id, a.user_id, a.server_id, a.name, a.description, a.deleted_at, a.embedding,
	ot.type, ot.access_token, ot.refresh_token, ot.expiry
FROM agents.mcp_accounts a
LEFT JOIN agents.oauth_tokens ot ON a.id = ot.account_id
WHERE a.id = sqlc.arg('account_id') AND a.deleted_at IS NULL;

-- GetAccountsBatch retrieves multiple accounts by ID in a single query.
-- Efficiently handles N+1 loading for users with multiple accounts.
--
-- Returns: List of accounts matched by IDs.
-- name: GetAccountsBatch :many
SELECT
	a.id, a.user_id, a.server_id, a.name, a.description, a.deleted_at, a.embedding,
	ot.type, ot.access_token, ot.refresh_token, ot.expiry
FROM agents.mcp_accounts a
LEFT JOIN agents.oauth_tokens ot ON a.id = ot.account_id
WHERE a.id = ANY(sqlc.arg('account_ids')::uuid[]) AND a.deleted_at IS NULL;

-- UpsertAccount creates or updates an MCP account.
-- Used when a user manually adds a server or updates settings.
-- RESETs deleted_at to NULL if the account was previously soft-deleted.
--
-- name: UpsertAccount :exec
INSERT INTO agents.mcp_accounts (id, user_id, server_id, name, description, deleted_at, embedding)
VALUES (
    sqlc.arg('id'),
    sqlc.arg('user_id'),
    sqlc.arg('server_id'),
    sqlc.arg('name'),
    sqlc.arg('description'),
    NULL,
    sqlc.arg('embedding')
)
ON CONFLICT (id) DO UPDATE
SET user_id = EXCLUDED.user_id,
    server_id = EXCLUDED.server_id,
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    embedding = EXCLUDED.embedding,
    deleted_at = NULL;

-- AddOAuthToken saves or updates OAuth credentials for an account.
-- Called after successful OAuth flow completions.
--
-- name: AddOAuthToken :exec
INSERT INTO agents.oauth_tokens (account_id, type, access_token, refresh_token, expiry)
VALUES (
    sqlc.arg('account_id'),
    sqlc.arg('type'),
    sqlc.arg('access_token'),
    sqlc.arg('refresh_token'),
    sqlc.arg('expiry')
)
ON CONFLICT (account_id) DO UPDATE SET
	type = EXCLUDED.type,
	access_token = EXCLUDED.access_token,
	refresh_token = EXCLUDED.refresh_token,
	expiry = EXCLUDED.expiry;

-- SoftDeleteAccount marks an account and all its tools as deleted without removing data.
-- Cascades soft-delete to associated tools automatically.
--
-- name: SoftDeleteAccount :exec
WITH deleted_account AS (
    UPDATE agents.mcp_accounts
    SET deleted_at = NOW()
    WHERE id = sqlc.arg('account_id')::UUID
    RETURNING id
)
UPDATE agents.mcp_tools
SET deleted_at = NOW()
WHERE account_id IN (SELECT id FROM deleted_account)
  AND deleted_at IS NULL;

-- InsertAccountTool adds a discovered tool to the account's catalog.
-- Used during tool discovery/sync proces.
-- RESETs deleted_at to NULL if tool existed previously.
--
-- name: InsertAccountTool :exec
INSERT INTO agents.mcp_tools (id, account_id, name, description, input, output, deleted_at, embedding)
VALUES (
    sqlc.arg('id'),
    sqlc.arg('account_id'),
    sqlc.arg('name'),
    sqlc.arg('description'),
    sqlc.arg('input'),
    sqlc.arg('output'),
    NULL,
    sqlc.arg('embedding')
)
ON CONFLICT (id) DO UPDATE
SET account_id = EXCLUDED.account_id,
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    input = EXCLUDED.input,
    output = EXCLUDED.output,
    embedding = EXCLUDED.embedding,
    deleted_at = NULL;

-- ListToolsForAccounts retrieves all active tools for a given set of accounts.
-- Used to load available tools into the agent's context window or toolbox.
--
-- Returns: All non-deleted tools belonging to valid accounts.
-- name: ListToolsForAccounts :many
SELECT t.id, t.account_id, t.name, t.description, t.input, t.output, t.embedding, t.deleted_at, a.name AS account_name
FROM agents.mcp_tools AS t
JOIN agents.mcp_accounts AS a ON t.account_id = a.id
WHERE t.account_id = ANY(sqlc.arg('account_ids')::uuid[]) AND t.deleted_at IS NULL;


-- DeleteAccountToken removes the OAuth token.
-- Used when revoking access or signing out of a specific integration.
-- NOTE: Hard delete is intentional for security tokens.
--
-- name: DeleteAccountToken :exec
DELETE FROM agents.oauth_tokens
WHERE account_id = sqlc.arg('account_id');

-- UpdateToolEmbedding updates the vector embedding for a specific tool.
-- Called asynchronously after tool discovery to enable semantic search.
--
-- name: UpdateToolEmbedding :exec
UPDATE agents.mcp_tools
SET embedding = sqlc.arg('embedding')
WHERE id = sqlc.arg('tool_id');

-- SearchToolsByEmbedding finds relevant tools using semantic similarity.
-- Core component of RAG: helps the agent pick the right tool for the job.
--
-- Returns: Tools ordered by similarity (closest first).
-- name: SearchToolsByEmbedding :many
SELECT t.id, t.account_id, t.name, t.description, t.input, t.output, t.embedding, t.deleted_at, a.name AS account_name,
       1 - (t.embedding <=> sqlc.arg('query_embedding')::vector) AS similarity
FROM agents.mcp_tools AS t
JOIN agents.mcp_accounts AS a ON t.account_id = a.id
WHERE t.deleted_at IS NULL
  AND t.account_id = ANY(sqlc.arg('account_ids')::uuid[])
ORDER BY t.embedding <=> sqlc.arg('query_embedding')::vector
LIMIT sqlc.arg('limit_count');

-- name: GetTool :one
SELECT t.id, t.account_id, t.name, t.description, t.input, t.output, t.embedding, t.deleted_at, a.name AS account_name
FROM agents.mcp_tools AS t
JOIN agents.mcp_accounts AS a ON t.account_id = a.id
WHERE t.id = sqlc.arg('tool_id') AND t.account_id = sqlc.arg('account_id') AND t.deleted_at IS NULL;

-- name: DeleteTool :exec
UPDATE agents.mcp_tools
SET deleted_at = NOW()
WHERE id = sqlc.arg('tool_id');

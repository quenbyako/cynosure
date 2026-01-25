-- AddServer registers a new MCP server endpoint or updates an existing one.
-- Does NOT config OAuth - use AddOAuthConfig for that.
-- RESETs deleted_at to NULL if server was previously soft-deleted.
--
-- name: AddServer :exec
INSERT INTO agents.mcp_servers (id, url, deleted_at, embedding)
VALUES (sqlc.arg('id')::UUID, sqlc.arg('url'), NULL, sqlc.narg('embedding'))
ON CONFLICT (id) DO UPDATE SET
	url = EXCLUDED.url,
	embedding = EXCLUDED.embedding,
	deleted_at = NULL;

-- AddOAuthConfig configures OAuth parameters for a server.
-- Often managed by admin or system configuration, not end-users.
--
-- name: AddOAuthConfig :exec
INSERT INTO agents.oauth_configs (server_id, client_id, client_secret, redirect_url, auth_url, token_url, expiration, scopes)
VALUES (
    sqlc.arg('server_id')::UUID,
    sqlc.arg('client_id'),
    sqlc.arg('client_secret'),
    sqlc.arg('redirect_url'),
    sqlc.arg('auth_url'),
    sqlc.arg('token_url'),
    sqlc.narg('expiration'),
    sqlc.arg('scopes')
)
ON CONFLICT (server_id) DO UPDATE SET
	client_id = EXCLUDED.client_id,
	client_secret = EXCLUDED.client_secret,
	redirect_url = EXCLUDED.redirect_url,
	auth_url = EXCLUDED.auth_url,
	token_url = EXCLUDED.token_url,
	expiration = EXCLUDED.expiration,
	scopes = EXCLUDED.scopes;

-- ListServers retrieves all available MCP servers with their OAuth configurations.
-- Used to display the catalog of available integrations to the user.
--
-- Returns: Flattened list of servers + oauth details.
-- name: ListServers :many
SELECT
	s.id, s.url, s.deleted_at, s.embedding,
	oc.client_id, oc.client_secret, oc.redirect_url, oc.auth_url, oc.token_url, oc.expiration, oc.scopes
FROM agents.mcp_servers s
LEFT JOIN agents.oauth_configs oc ON s.id = oc.server_id
WHERE s.deleted_at IS NULL
ORDER BY s.id;

-- name: GetServerInfo :one
SELECT
	s.id, s.url, s.deleted_at, s.embedding,
	oc.client_id, oc.client_secret, oc.redirect_url, oc.auth_url, oc.token_url, oc.expiration, oc.scopes
FROM agents.mcp_servers s
LEFT JOIN agents.oauth_configs oc ON s.id = oc.server_id
WHERE s.id = sqlc.arg('id')::UUID AND s.deleted_at IS NULL;

-- LookupByURL finds a server by its base URL.
-- Key Use Case: Preventing duplicate server entries when user pastes a URL.
--
-- Parameters:
--   url: url (exact match)
-- name: LookupByURL :one
SELECT
	s.id, s.url, s.deleted_at, s.embedding,
	oc.client_id, oc.client_secret, oc.redirect_url, oc.auth_url, oc.token_url, oc.expiration, oc.scopes
FROM agents.mcp_servers s
LEFT JOIN agents.oauth_configs oc ON s.id = oc.server_id
WHERE s.url = sqlc.arg('url') AND s.deleted_at IS NULL;

-- DeleteServer soft-deletes a server definition.
-- Existing user accounts connected to this server will remain but may become orphaned
-- unless handled by app logic.
-- name: DeleteServer :exec
UPDATE agents.mcp_servers
SET deleted_at = NOW()
WHERE id = sqlc.arg('id')::UUID;

-- name: AddServer :exec
INSERT INTO agents.mcp_servers (id, url)
VALUES ($1, $2)
ON CONFLICT (id) DO UPDATE SET
	url = EXCLUDED.url,
	deleted_at = NULL;

-- name: AddOAuthConfig :exec
INSERT INTO agents.oauth_configs (server_id, client_id, client_secret, redirect_url, auth_url, token_url, expiration, scopes)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (server_id) DO UPDATE SET
	client_id = EXCLUDED.client_id,
	client_secret = EXCLUDED.client_secret,
	redirect_url = EXCLUDED.redirect_url,
	auth_url = EXCLUDED.auth_url,
	token_url = EXCLUDED.token_url,
	expiration = EXCLUDED.expiration,
	scopes = EXCLUDED.scopes;

-- name: ListServers :many
SELECT
	s.id, s.url,
	oc.client_id, oc.client_secret, oc.redirect_url, oc.auth_url, oc.token_url, oc.expiration, oc.scopes
FROM agents.mcp_servers s
LEFT JOIN agents.oauth_configs oc ON s.id = oc.server_id
WHERE s.deleted_at IS NULL
ORDER BY s.id;

-- name: GetServerInfo :one
SELECT
	s.id, s.url,
	oc.client_id, oc.client_secret, oc.redirect_url, oc.auth_url, oc.token_url, oc.expiration, oc.scopes
FROM agents.mcp_servers s
LEFT JOIN agents.oauth_configs oc ON s.id = oc.server_id
WHERE s.id = $1 AND s.deleted_at IS NULL;

-- name: LookupByURL :one
SELECT
	s.id, s.url,
	oc.client_id, oc.client_secret, oc.redirect_url, oc.auth_url, oc.token_url, oc.expiration, oc.scopes
FROM agents.mcp_servers s
LEFT JOIN agents.oauth_configs oc ON s.id = oc.server_id
WHERE s.url = $1 AND s.deleted_at IS NULL;

-- name: DeleteServer :exec
UPDATE agents.mcp_servers
SET deleted_at = NOW()
WHERE id = $1;

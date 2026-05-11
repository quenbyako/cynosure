-- name: GetUserQuota :one
SELECT
    chat_input_period,
    chat_input_limit,
    chat_output_period,
    chat_output_limit,
    embedding_period,
    embedding_limit,
    max_await_period,
    agents_limit,
    mcp_accounts_limit
FROM agents.user_quotas
WHERE user_id = $1;

-- name: GetOrCreateBucket :one
INSERT INTO agents.rate_limit_buckets (user_id, resource_type, last_leak_at)
VALUES ($1, $2, $3)
ON CONFLICT (user_id, resource_type) DO UPDATE SET user_id = EXCLUDED.user_id
RETURNING level, last_leak_at;

-- name: UpdateBucket :exec
UPDATE agents.rate_limit_buckets
SET level = $1, last_leak_at = $2
WHERE user_id = $3 AND resource_type = $4;

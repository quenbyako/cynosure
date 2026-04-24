-- ListAgentSettings retrieves all configured agent settings profiles.
-- Used for admin dashboards or selection menus.
--
-- Returns: All settings ordered by model name.
-- name: ListAgentSettings :many
SELECT id, user_id, model, system_message, temperature, top_p, max_context, stop_words
FROM agents.agent_settings
WHERE user_id = sqlc.arg('user_id')::UUID
ORDER BY model;

-- GetAgentSettings retrieves a specific configuration profile.
-- Critical for the Agent Loop: loaded before processing messages to configure the LLM.
--
-- name: GetAgentSettings :one
SELECT id, user_id, model, system_message, temperature, top_p, max_context, stop_words
FROM agents.agent_settings
WHERE id = sqlc.arg('id')::UUID;

-- UpsertAgentSettings creates or updates a configuration profile.
-- Used when creating a new agent persona or tuning parameters.
--
-- name: UpsertAgentSettings :exec
INSERT INTO agents.agent_settings (id, user_id, model, system_message, temperature, top_p, max_context, stop_words)
VALUES (
    sqlc.arg('id')::UUID,
    sqlc.arg('user_id')::UUID,
    sqlc.arg('model'),
    sqlc.arg('system_message'),
    sqlc.arg('temperature'),
    sqlc.arg('top_p'),
    sqlc.arg('max_context'),
    sqlc.narg('stop_words')
)
ON CONFLICT (id) DO UPDATE SET
	model = EXCLUDED.model,
	system_message = EXCLUDED.system_message,
	temperature = EXCLUDED.temperature,
	top_p = EXCLUDED.top_p,
	max_context = EXCLUDED.max_context,
	stop_words = EXCLUDED.stop_words;

-- DeleteAgentSettings removes a configuration profile.
-- HARD delete allowed here as settings are lightweight configuration.
--
-- name: DeleteAgentSettings :exec
DELETE FROM agents.agent_settings
WHERE id = sqlc.arg('id')::UUID;

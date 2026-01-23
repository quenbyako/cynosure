-- name: ListModelSettings :many
SELECT id, model, system_message, temperature, top_p, stop_words
FROM agents.model_settings
ORDER BY model;

-- name: GetModelSettings :one
SELECT id, model, system_message, temperature, top_p, stop_words
FROM agents.model_settings
WHERE id = $1;

-- name: UpsertModelSettings :exec
INSERT INTO agents.model_settings (id, model, system_message, temperature, top_p, stop_words)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (id) DO UPDATE SET
	model = EXCLUDED.model,
	system_message = EXCLUDED.system_message,
	temperature = EXCLUDED.temperature,
	top_p = EXCLUDED.top_p,
	stop_words = EXCLUDED.stop_words;

-- name: DeleteModelSettings :exec
DELETE FROM agents.model_settings
WHERE id = $1;

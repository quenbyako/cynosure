-- CreateThread creates a new chat thread (Aggregate Root in DDD terms).
-- Initializes last_message_pos to 0, which serves as the version counter
-- for Optimistic Concurrency Control when appending messages.
--
-- Usage: Call this once when starting a new conversation.
-- name: CreateThread :exec
INSERT INTO agents.threads (id, user_id, last_message_pos, created_at)
VALUES (sqlc.arg(id), sqlc.arg(user_id), 0, NOW());

-- GetThreadWithMessages retrieves thread metadata AND all messages in a single query.
-- This combines what were previously GetThread + GetThreadMessages for efficiency.
--
-- Returns:
--   First row contains thread metadata (id, last_message_pos, created_at)
--   Subsequent rows contain messages with type-specific data via LEFT JOINs
--
-- The last_message_pos field is critical for Optimistic Concurrency Control.
-- If the thread has no messages, returns a single row with thread metadata
-- and NULL message fields.
--
-- Usage: Call when loading a chat thread. Check last_message_pos before
-- adding new messages to implement OCC.
-- name: GetThreadWithMessages :many
SELECT
    t.id AS thread_id,
    t.user_id,
    t.last_message_pos,
    t.created_at AS thread_created_at,
    m.position,
    m.msg_type,
    m.merge_tag,
    m.created_at AS message_created_at,
    u.content AS user_content,
    a.text AS assistant_text,
    a.reasoning AS assistant_reasoning,
    a.agent_id::UUID AS assistant_agent_id,
    tr.tool_id::UUID AS req_tool_id,
    tr.tool_name AS req_tool_name,
    tr.tool_call_id AS req_tool_call_id,
    tr.reasoning AS req_reasoning,
    tr.arguments AS req_arguments,
    ts.request_position AS result_request_position,
    ts.tool_call_id AS result_tool_call_id,
    ts.is_error AS result_is_error,
    ts.content AS result_content,
    tr_origin.tool_name AS result_tool_name
FROM agents.threads t
LEFT JOIN agents.messages m ON t.id = m.thread_id
LEFT JOIN agents.messages_user u
    ON m.thread_id = u.thread_id AND m.position = u.position
LEFT JOIN agents.messages_assistant a
    ON m.thread_id = a.thread_id AND m.position = a.position
LEFT JOIN agents.messages_tool_request tr
    ON m.thread_id = tr.thread_id AND m.position = tr.position
LEFT JOIN agents.messages_tool_result ts
    ON m.thread_id = ts.thread_id AND m.position = ts.position
LEFT JOIN agents.messages_tool_request tr_origin
    ON ts.thread_id = tr_origin.thread_id AND ts.request_position = tr_origin.position
WHERE t.id = sqlc.arg(id)
ORDER BY m.position ASC NULLS FIRST;

-- InsertMessageUser atomically inserts a user message and updates thread position.
-- Uses CTEs to perform UPDATE (with OCC check), then INSERT into messages and messages_user
-- in a SINGLE SQL statement.
--
-- Parameters:
--   thread_id
--   position (must equal current last_message_pos + 1)
--   current_last_message_pos (for OCC check in UPDATE)
--   merge_tag (for grouping related messages, typically 0)
--   content (user's message text)
--
-- If the UPDATE affects 0 rows (OCC conflict), the entire statement does nothing
-- and returns 0 rows - caller must check RowsAffected and retry if needed.
--
-- Transaction flow (all in one statement):
--   1. UPDATE threads with OCC check (if fails, stop here)
--   2. INSERT into messages (base table)
--   3. INSERT into messages_user (type-specific)
--
-- Usage: Single atomic operation to insert user message with OCC.
-- name: InsertMessageUser :execrows
WITH update_thread AS (
    UPDATE agents.threads
    SET last_message_pos = sqlc.arg(position)::BIGINT
    WHERE id = sqlc.arg(thread_id)::TEXT AND last_message_pos = sqlc.arg(current_last_message_pos)::BIGINT
    RETURNING id
),
msg AS (
    INSERT INTO agents.messages (thread_id, position, msg_type, merge_tag, created_at)
    SELECT sqlc.arg(thread_id)::TEXT, sqlc.arg(position)::BIGINT, 'user', sqlc.arg(merge_tag)::BIGINT, NOW()
    FROM update_thread
    RETURNING thread_id, position
)
INSERT INTO agents.messages_user (thread_id, position, content)
SELECT thread_id, position, sqlc.arg(content) FROM msg;

-- InsertMessageAssistant atomically inserts an assistant message and updates thread position.
-- Uses CTEs to perform all operations in a single SQL statement with OCC.
--
-- Parameters:
--   thread_id
--   position (must equal current last_message_pos + 1)
--   current_last_message_pos (for OCC check)
--   merge_tag
--   text (assistant's response, may be empty if reasoning is present)
--   reasoning (chain-of-thought, may be empty if text is present)
--   agent_id (reference to agent_settings)
--
-- Note: CHECK constraint ensures at least one of (text, reasoning) is non-empty.
-- This allows the model to "think" without necessarily responding.
-- name: InsertMessageAssistant :execrows
WITH update_thread AS (
    UPDATE agents.threads
    SET last_message_pos = sqlc.arg(position)::BIGINT
    WHERE id = sqlc.arg(thread_id)::TEXT AND last_message_pos = sqlc.arg(current_last_message_pos)::BIGINT
    RETURNING id
),
msg AS (
    INSERT INTO agents.messages (thread_id, position, msg_type, merge_tag, created_at)
    SELECT sqlc.arg(thread_id)::TEXT, sqlc.arg(position)::BIGINT, 'assistant', sqlc.arg(merge_tag)::BIGINT, NOW()
    FROM update_thread
    RETURNING thread_id, position
)
INSERT INTO agents.messages_assistant (thread_id, position, text, reasoning, agent_id)
SELECT thread_id, position, sqlc.arg(text), sqlc.arg(reasoning), sqlc.arg(agent_id)::UUID FROM msg;

-- InsertMessageToolRequest atomically inserts a tool request and updates thread position.
-- Uses CTEs to perform all operations in a single SQL statement with OCC.
--
-- Parameters:
--   thread_id
--   position
--   current_last_message_pos (for OCC)
--   merge_tag
--   tool_id (FK to mcp_tools, must never be inserted as null)
--   tool_name (denormalized - preserves historical name even if tool renamed)
--   tool_call_id (unique identifier for matching with result)
--   reasoning (model's thinking before calling tool)
--   arguments (JSONB object with tool parameters)
--
-- Note: tool_name is intentionally denormalized to create an immutable
-- historical record of the exact tool name at invocation time.
-- name: InsertMessageToolRequest :execrows
WITH update_thread AS (
    UPDATE agents.threads
    SET last_message_pos = sqlc.arg(position)::BIGINT
    WHERE id = sqlc.arg(thread_id)::TEXT AND last_message_pos = sqlc.arg(current_last_message_pos)::BIGINT
    RETURNING id
),
msg AS (
    INSERT INTO agents.messages (thread_id, position, msg_type, merge_tag, created_at)
    SELECT sqlc.arg(thread_id)::TEXT, sqlc.arg(position)::BIGINT, 'tool_request', sqlc.arg(merge_tag)::BIGINT, NOW()
    FROM update_thread
    RETURNING thread_id, position
)
INSERT INTO agents.messages_tool_request (thread_id, position, tool_id, tool_name, tool_call_id, reasoning, arguments)
SELECT thread_id, position, sqlc.narg(tool_id)::UUID, sqlc.arg(tool_name), sqlc.arg(tool_call_id), sqlc.arg(reasoning), sqlc.arg(arguments) FROM msg;

-- name: GetToolRequestPosition :one
SELECT position FROM agents.messages_tool_request
WHERE thread_id = sqlc.arg(thread_id) AND tool_call_id = sqlc.arg(tool_call_id);

-- InsertMessageToolResult atomically inserts a tool execution result and updates thread position.
-- Uses CTEs to perform all operations in a single SQL statement with OCC.
--
-- Parameters:
--   thread_id
--   position (this message's position, NOT the request position)
--   current_last_message_pos (for OCC)
--   merge_tag
--   request_position (FK to the original tool_request message)
--   tool_call_id (must match the request's tool_call_id)
--   is_error (false for success, true for errors)
--   content (JSONB - either success result or error details)
--
-- The request_position FK with ON DELETE CASCADE ensures referential integrity.
-- name: InsertMessageToolResult :execrows
WITH update_thread AS (
    UPDATE agents.threads
    SET last_message_pos = sqlc.arg(position)::BIGINT
    WHERE id = sqlc.arg(thread_id)::TEXT AND last_message_pos = sqlc.arg(current_last_message_pos)::BIGINT
    RETURNING id
),
msg AS (
    INSERT INTO agents.messages (thread_id, position, msg_type, merge_tag, created_at)
    SELECT sqlc.arg(thread_id)::TEXT, sqlc.arg(position)::BIGINT, 'tool_result', sqlc.arg(merge_tag)::BIGINT, NOW()
    FROM update_thread
    RETURNING thread_id, position
)
INSERT INTO agents.messages_tool_result (thread_id, position, request_position, tool_call_id, is_error, content)
SELECT thread_id, position, sqlc.arg(request_position)::BIGINT, sqlc.arg(tool_call_id), sqlc.arg(is_error), sqlc.arg(content) FROM msg;

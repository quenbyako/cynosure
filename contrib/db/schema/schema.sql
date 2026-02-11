CREATE EXTENSION IF NOT EXISTS "vector";
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE SCHEMA IF NOT EXISTS agents;

-- =============================================================================
-- TABLES
-- =============================================================================

CREATE TABLE agents.users (
	-- Note: for now that's only what domain model requires, so leave it with
	-- only field.
	id UUID PRIMARY KEY
);

-- user_telegram table is related to Telegram integration.
--
-- Note: for now, that's only telegram id mapping, but later there will be more
-- info, like username, first name, last name, etc.
CREATE TABLE agents.user_telegram (
	user_id     UUID   NOT NULL PRIMARY KEY,
	telegram_id BIGINT NOT NULL UNIQUE CHECK (telegram_id > 0)
);

CREATE TABLE agents.mcp_accounts (
	id         UUID PRIMARY KEY,
	user_id    UUID NOT NULL,
	server_id  UUID NOT NULL,
	deleted_at TIMESTAMPTZ,

	name        TEXT NOT NULL,
	description TEXT NOT NULL,
	-- using pgvector, it's allowed to have empty embeddings since flow of
	-- adding embeddings is optional.
	--
	-- currently — using only gemini with fixed vector, however, when the model
	-- or configs will change — we will add a new column with new vector config.
	-- Not perfect — but for now it's fine.
	embedding VECTOR(1536)
);

CREATE TABLE agents.mcp_servers (
	id         UUID PRIMARY KEY,
	deleted_at TIMESTAMPTZ,

	url        TEXT NOT NULL UNIQUE,
	-- using pgvector, it's allowed to have empty embeddings since flow of
	-- adding embeddings is optional.
	embedding VECTOR(1536)
);

CREATE TABLE agents.mcp_tools (
	id         UUID  PRIMARY KEY,
	account_id UUID  NOT NULL,
	deleted_at TIMESTAMPTZ,

	name      TEXT  NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	input     JSONB NOT NULL,
	output    JSONB NOT NULL,
	-- using pgvector, it's allowed to have empty embeddings since flow of
	-- adding embeddings is optional.
	--
	-- currently — using only gemini with fixed vector, however, when the model
	-- or configs will change — we will add a new column with new vector config.
	-- Not perfect — but for now it's fine.
	embedding VECTOR(1536)
);

CREATE TABLE agents.agent_settings (
	id             UUID PRIMARY KEY,
	model          TEXT NOT NULL,
	system_message TEXT NOT NULL,
	temperature    REAL NOT NULL CHECK (temperature >= 0), -- zero value counts as unset
	top_p          REAL NOT NULL CHECK (top_p >= 0),       -- zero value counts as unset
	stop_words     TEXT[]
);

CREATE TABLE agents.oauth_configs (
	server_id     UUID   PRIMARY KEY,
	client_id     TEXT   NOT NULL,
	client_secret TEXT   NOT NULL,
	redirect_url  TEXT   NOT NULL,
	scopes        TEXT[] NOT NULL CHECK (cardinality(scopes) >= 0),
	auth_url      TEXT   NOT NULL,
	token_url     TEXT   NOT NULL,
	expiration    TIMESTAMPTZ -- configs MAY expire, if this config created through MCP flow, and not attached to dev account
);

CREATE TABLE agents.oauth_tokens (
	account_id    UUID PRIMARY KEY,
	type          TEXT,
	access_token  TEXT NOT NULL,
	refresh_token TEXT,
	expiry        TIMESTAMPTZ
);

CREATE TABLE agents.threads (
	-- using TEXT instead of UUID cause different messengers may interpret
	-- thread id differently. In telegram, for example, message_thread_id is an
	-- integer, as well as user id.
	id               TEXT        PRIMARY KEY,
	user_id          UUID        NOT NULL,
	created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	-- last_message_pos is used for Optimistic Concurrency Control
	last_message_pos BIGINT NOT NULL DEFAULT 0
);

-- Messages table (Value Objects inside Thread aggregate)
CREATE TABLE agents.messages (
	thread_id  TEXT        NOT NULL,
	position   BIGINT      NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

	msg_type   TEXT NOT NULL CHECK (msg_type IN ('user', 'assistant', 'tool_request', 'tool_result')),
	merge_tag  BIGINT NOT NULL DEFAULT 0,

	PRIMARY KEY (thread_id, position)
);

CREATE TABLE agents.messages_user (
	thread_id TEXT   NOT NULL,
	position  BIGINT NOT NULL,
	content   TEXT   NOT NULL CHECK (length(content) > 0),

	PRIMARY KEY (thread_id, position)
);

CREATE TABLE agents.messages_assistant (
	thread_id TEXT   NOT NULL,
	position  BIGINT NOT NULL,
	text      TEXT   NOT NULL,
	reasoning TEXT   NOT NULL DEFAULT '', -- reasoning before generating this message. Models MAY think before running tool, in that case, reasoning stores there in tool_call
	agent_id  UUID   NOT NULL,

	PRIMARY KEY (thread_id, position),
    -- Case when model will think but won't answer anything:
	--
	-- If model did call some tools, then thought about something, and decided that there is no need to answer — then text will be empty, but reasoning will be non-empty.
	CHECK (length(text) > 0 OR length(reasoning) > 0)
);

CREATE TABLE agents.messages_tool_request (
	thread_id    TEXT   NOT NULL,
	position     BIGINT NOT NULL,

	tool_id      UUID, -- must be null ONLY IN CASE WHERE TOOL IS DELETED.
	tool_name    TEXT NOT NULL, -- for simpler reconstruction without joining to mcp_tools
	tool_call_id TEXT NOT NULL,

	reasoning    TEXT NOT NULL DEFAULT '', -- when model doesn't make any messages, but just thinks before calling tool — this reasoning stores here.
	arguments    JSONB NOT NULL CHECK (jsonb_typeof(arguments) = 'object'),

	PRIMARY KEY (thread_id, position)
);

CREATE TABLE agents.messages_tool_result (
	thread_id        TEXT   NOT NULL,
	position         BIGINT NOT NULL,
	request_position BIGINT NOT NULL,
	tool_call_id     TEXT   NOT NULL,

	is_error         BOOLEAN NOT NULL DEFAULT FALSE,
	content          JSONB NOT NULL,

	PRIMARY KEY (thread_id, position)
);

-- =============================================================================
-- INDEXES
-- =============================================================================

CREATE INDEX idx_messages_thread_pos ON agents.messages(thread_id, position DESC);
CREATE INDEX idx_messages_type ON agents.messages(thread_id, msg_type);
CREATE INDEX idx_tools_account ON agents.mcp_tools(account_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_accounts_user ON agents.mcp_accounts(user_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_tool_result_request ON agents.messages_tool_result(thread_id, request_position);
CREATE INDEX idx_user_telegram ON agents.user_telegram(telegram_id);

-- =============================================================================
-- FOREIGN KEYS
-- =============================================================================

-- MCP hierarchy: users -> accounts -> tools
ALTER TABLE agents.mcp_accounts ADD CONSTRAINT fk_account_user
	FOREIGN KEY (user_id) REFERENCES agents.users(id)
	ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE agents.user_telegram ADD CONSTRAINT fk_user_telegram_user
	FOREIGN KEY (user_id) REFERENCES agents.users(id)
	ON DELETE CASCADE ON UPDATE RESTRICT;

ALTER TABLE agents.mcp_accounts ADD CONSTRAINT fk_account_server
	FOREIGN KEY (server_id) REFERENCES agents.mcp_servers(id)
	ON DELETE RESTRICT ON UPDATE RESTRICT;

ALTER TABLE agents.mcp_tools ADD CONSTRAINT fk_tool_account
	FOREIGN KEY (account_id) REFERENCES agents.mcp_accounts(id)
	ON DELETE RESTRICT ON UPDATE RESTRICT;

-- OAuth
ALTER TABLE agents.oauth_configs ADD CONSTRAINT fk_oauth_config_server
	FOREIGN KEY (server_id) REFERENCES agents.mcp_servers(id)
	ON DELETE CASCADE ON UPDATE RESTRICT;

ALTER TABLE agents.oauth_tokens ADD CONSTRAINT fk_oauth_token_account
	FOREIGN KEY (account_id) REFERENCES agents.mcp_accounts(id)
	ON DELETE CASCADE ON UPDATE RESTRICT;

-- Messages hierarchy: threads -> messages -> message_types
ALTER TABLE agents.messages ADD CONSTRAINT fk_message_thread
	FOREIGN KEY (thread_id) REFERENCES agents.threads(id)
	ON DELETE CASCADE ON UPDATE RESTRICT;

ALTER TABLE agents.messages_user ADD CONSTRAINT fk_message_user_base
	FOREIGN KEY (thread_id, position) REFERENCES agents.messages(thread_id, position)
	ON DELETE CASCADE ON UPDATE CASCADE;

ALTER TABLE agents.messages_assistant ADD CONSTRAINT fk_message_assistant_base
	FOREIGN KEY (thread_id, position) REFERENCES agents.messages(thread_id, position)
	ON DELETE CASCADE ON UPDATE CASCADE;

ALTER TABLE agents.messages_assistant ADD CONSTRAINT fk_message_assistant_agent
	FOREIGN KEY (agent_id) REFERENCES agents.agent_settings(id)
	ON DELETE SET NULL ON UPDATE RESTRICT;

ALTER TABLE agents.messages_tool_request ADD CONSTRAINT fk_message_tool_request_base
	FOREIGN KEY (thread_id, position) REFERENCES agents.messages(thread_id, position)
	ON DELETE CASCADE ON UPDATE CASCADE;

ALTER TABLE agents.messages_tool_request ADD CONSTRAINT fk_message_tool_request_tool
	FOREIGN KEY (tool_id) REFERENCES agents.mcp_tools(id)
	ON DELETE SET NULL ON UPDATE RESTRICT;

ALTER TABLE agents.messages_tool_result ADD CONSTRAINT fk_message_tool_result_base
	FOREIGN KEY (thread_id, position) REFERENCES agents.messages(thread_id, position)
	ON DELETE CASCADE ON UPDATE CASCADE;

ALTER TABLE agents.messages_tool_result ADD CONSTRAINT fk_message_tool_result_request
	FOREIGN KEY (thread_id, request_position) REFERENCES agents.messages_tool_request(thread_id, position)
	ON DELETE CASCADE ON UPDATE CASCADE;

ALTER TABLE agents.threads ADD CONSTRAINT fk_thread_user
	FOREIGN KEY (user_id) REFERENCES agents.users(id)
	ON DELETE CASCADE ON UPDATE RESTRICT;

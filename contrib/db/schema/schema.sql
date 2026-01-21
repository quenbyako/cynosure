CREATE SCHEMA IF NOT EXISTS agents;

CREATE TABLE agents.users (
	id        UUID PRIMARY KEY
	-- note; for now that's only what domain model requires, so leave it with
	-- only field.
);

CREATE TABLE agents.mcp_accounts (
	id            UUID PRIMARY KEY,
	user_id       UUID NOT NULL,
	server        UUID NOT NULL,
	name          TEXT NOT NULL,
	description   TEXT NOT NULL,
	token         UUID,
	deleted_at    TIMESTAMP
);

CREATE TABLE agents.mcp_servers (
	id           UUID PRIMARY KEY,
	url          TEXT NOT NULL UNIQUE,
	deleted_at   TIMESTAMP
);

CREATE TABLE agents.mcp_tools (
	id      UUID PRIMARY KEY,
	account UUID NOT NULL,
	name    TEXT NOT NULL,
	input   JSONB NOT NULL,
	output  JSONB NOT NULL
);

CREATE TABLE agents.model_settings (
	id             UUID PRIMARY KEY,
	model          TEXT NOT NULL,
	system_message TEXT NOT NULL,
	temperature    numeric,
	top_p          numeric,
	stop_words     TEXT[]
);

CREATE TABLE agents.oauth_configs (
	server_id     UUID PRIMARY KEY,
	client_id     TEXT NOT NULL,
	client_secret TEXT NOT NULL,
	redirect_url  TEXT NOT NULL,
	scopes 		  TEXT[] NOT NULL, -- must be at least zero-length array, but not null
	auth_url      TEXT NOT NULL,
	token_url     TEXT NOT NULL,
	expiration    TIMESTAMP
);

CREATE TABLE agents.oauth_tokens (
	id            UUID PRIMARY KEY,
	type          TEXT,
	access_token  TEXT NOT NULL,
	refresh_token TEXT,
	expiry        TIMESTAMP
);

ALTER TABLE agents.mcp_accounts ADD CONSTRAINT "account_server" FOREIGN KEY ("server") REFERENCES agents.mcp_servers("id") ON DELETE RESTRICT ON UPDATE RESTRICT;
ALTER TABLE agents.mcp_accounts ADD CONSTRAINT "account_token" FOREIGN KEY ("token") REFERENCES agents.oauth_tokens("id") ON DELETE RESTRICT ON UPDATE RESTRICT;
ALTER TABLE agents.mcp_accounts ADD CONSTRAINT "user_accounts" FOREIGN KEY ("user_id") REFERENCES agents.users("id") ON DELETE RESTRICT ON UPDATE RESTRICT;
ALTER TABLE agents.oauth_configs ADD CONSTRAINT "auth_config_server" FOREIGN KEY ("server_id") REFERENCES agents.mcp_servers("id") ON DELETE CASCADE ON UPDATE RESTRICT;
ALTER TABLE agents.mcp_tools ADD CONSTRAINT "account_tools" FOREIGN KEY ("account") REFERENCES agents.mcp_accounts("id") ON DELETE RESTRICT ON UPDATE RESTRICT;

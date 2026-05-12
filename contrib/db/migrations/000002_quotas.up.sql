
CREATE TABLE agents.plans (
	id					UUID     PRIMARY KEY,
	chat_input_period   INTERVAL NOT NULL CHECK (chat_input_period > '0s'::INTERVAL),
	chat_input_limit    INT      NOT NULL CHECK (chat_input_limit >= 0),
	chat_output_period  INTERVAL NOT NULL CHECK (chat_output_period > '0s'::INTERVAL),
	chat_output_limit   INT      NOT NULL CHECK (chat_output_limit >= 0),
	embedding_period    INTERVAL NOT NULL CHECK (embedding_period > '0s'::INTERVAL),
	embedding_limit     INT      NOT NULL CHECK (embedding_limit >= 0),
	max_await_period    INTERVAL NOT NULL CHECK (max_await_period >= '0s'::INTERVAL),
	agents_limit        INT      NOT NULL CHECK (agents_limit >= 0),
	mcp_accounts_limit  INT      NOT NULL CHECK (mcp_accounts_limit >= 0)
);

CREATE TABLE agents.user_plans (
	user_id UUID PRIMARY KEY,
	plan_id UUID NOT NULL REFERENCES agents.plans(id)
);

CREATE TABLE agents.rate_limit_buckets (
	user_id       UUID NOT NULL,
	resource_type TEXT NOT NULL, -- 'input', 'output', 'embedding'
	level         DOUBLE PRECISION NOT NULL DEFAULT 0,
	last_leak_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

	PRIMARY KEY (user_id, resource_type)
) WITH (FILLFACTOR = 70);


CREATE VIEW agents.user_quotas AS
	SELECT
		up.user_id,
		p.id AS plan_id,
		p.chat_input_period,
		p.chat_input_limit,
		p.chat_output_period,
		p.chat_output_limit,
		p.embedding_period,
		p.embedding_limit,
		p.max_await_period,
		p.agents_limit,
		p.mcp_accounts_limit
	FROM agents.user_plans up
	JOIN agents.plans p ON up.plan_id = p.id;

GRANT INSERT, DELETE, SELECT, UPDATE ON TABLE agents.plans TO cynosure;
GRANT INSERT, DELETE, SELECT, UPDATE ON TABLE agents.user_plans TO cynosure;
GRANT INSERT, DELETE, SELECT, UPDATE ON TABLE agents.rate_limit_buckets TO cynosure;

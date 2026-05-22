CREATE UNIQUE INDEX CONCURRENTLY idx_accounts_user_name ON agents.mcp_accounts USING btree (user_id, name) WHERE (deleted_at IS NULL);

ALTER TABLE "agents"."plans" ADD COLUMN "plan_key" text COLLATE "pg_catalog"."default" NOT NULL;

ALTER TABLE "agents"."plans" ADD CONSTRAINT "plans_plan_key_check" CHECK((length(plan_key) > 0)) NOT VALID;

ALTER TABLE "agents"."plans" VALIDATE CONSTRAINT "plans_plan_key_check";

CREATE UNIQUE INDEX CONCURRENTLY plans_plan_key_key ON agents.plans USING btree (plan_key);

ALTER TABLE "agents"."plans" ADD CONSTRAINT "plans_plan_key_key" UNIQUE USING INDEX "plans_plan_key_key";


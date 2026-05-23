DROP INDEX CONCURRENTLY "agents"."idx_accounts_user_name";

ALTER TABLE "agents"."plans" DROP CONSTRAINT "plans_plan_key_key";

ALTER TABLE "agents"."plans" DROP CONSTRAINT "plans_plan_key_check";

ALTER TABLE "agents"."plans" DROP COLUMN "plan_key";


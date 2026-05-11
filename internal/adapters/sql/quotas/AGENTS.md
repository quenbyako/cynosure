# Quotas Adapter

This package provides a PostgreSQL-backed implementation of the rate-limiting ports.

## Rules

- All rate-limiting state must be persisted in the `agents.rate_limit_buckets` table.
- Use atomic SQL transactions for composite quota checks (e.g., input + output).
- Ensure "leak" time and "debt capping" are committed even if a request is blocked, to maintain consistent wait times.
- Follow the Leaky Bucket algorithm with debt support and wait-time capping.
- If a user has no assigned plan, fall back to a hardcoded "Trial" plan (1000 tokens/min) to prevent service denial.

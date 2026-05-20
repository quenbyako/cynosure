package sql_test

import (
	"context"
	"fmt"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	accountsTests "github.com/quenbyako/cynosure/internal/domains/cynosure/ports/accounts/testsuite"
	rlport "github.com/quenbyako/cynosure/internal/domains/cynosure/ports/ratelimiter"
	ratelimiter "github.com/quenbyako/cynosure/internal/domains/cynosure/ports/ratelimiter/testsuite"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/testsuite"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"

	. "github.com/quenbyako/cynosure/internal/adapters/sql"
)

func TestAdapter(t *testing.T) {
	pool := SetupTestDB(t)

	// Get connection string from pool for NewAdapter
	connStr := pool.Config().ConnString()
	planID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	adapter, err := New(t.Context(), must(url.Parse(connStr)),
		WithDefaultPlanID(planID),
	)
	require.NoError(t, err, "Failed to create SQL adapter")
	require.NotNil(t, adapter, "Adapter should not be nil")

	t.Cleanup(func() {
		if closeErr := adapter.Close(); closeErr != nil {
			t.Errorf("failed to close adapter: %v", closeErr)
		}
	})

	t.Run("Accounts", accountsTests.Run(adapter.Accounts(),
		accountsTests.WithSaveAccountSeeder(seeder(pool)),
		accountsTests.WithAccountStorageCleanup(cleaner(pool)),
	))

	t.Run("ModelSettings", testsuite.RunModelSettingsStorageTests(adapter,
		testsuite.WithModelSettingsStorageCleanup(cleaner(pool)),
	))

	t.Run("Servers", testsuite.RunServerStorageTests(adapter,
		testsuite.WithServerStorageCleanup(cleaner(pool)),
	))

	t.Run("RateLimiter", ratelimiter.Run(func(
		ctx context.Context, params ratelimiter.SetupParams,
	) (rlport.Port, error) {
		// Insert plan
		_, err := pool.Exec(ctx, `
			INSERT INTO agents.plans (
				id, plan_key, chat_input_period, chat_input_limit,
				chat_output_period, chat_output_limit,
				embedding_period, embedding_limit,
				max_await_period, agents_limit, mcp_accounts_limit
			) VALUES (
				$1, 'test-plan', $2, $3, $4, $5, $6, $7, $8, 100, 100
			)
		`,
			planID,
			pgtype.Interval{Microseconds: params.ChatInput.Period.Microseconds(), Valid: true},
			params.ChatInput.Limit,
			pgtype.Interval{Microseconds: params.ChatOutput.Period.Microseconds(), Valid: true},
			params.ChatOutput.Limit,
			pgtype.Interval{Microseconds: params.EmbeddingInput.Period.Microseconds(), Valid: true},
			params.EmbeddingInput.Limit,
			pgtype.Interval{Microseconds: params.MaxWait.Microseconds(), Valid: true},
		)
		if err != nil {
			return nil, fmt.Errorf("seeding plan: %w", err)
		}

		rl, err := adapter.WithClock(params.Now).RateLimiter()
		if err != nil {
			return nil, fmt.Errorf("getting rate limiter: %w", err)
		}

		return &testRateLimiter{Port: rl, pool: pool, planID: planID}, nil
	},
		ratelimiter.WithCleanup(cleaner(pool)),
	))
}

// custom wrapper, intended only to provide plans for all users that are
// provided from testsuite
type testRateLimiter struct {
	rlport.Port
	pool   *pgxpool.Pool
	planID uuid.UUID
}

func (t *testRateLimiter) ConsumeChatRequests(
	ctx context.Context, user ids.UserID, model string, inputTokens int,
) (rlport.ConsumedTokensFunc, error) {
	if err := t.ensurePlan(ctx, user); err != nil {
		return nil, fmt.Errorf("ensuring plan: %w", err)
	}

	//nolint:wrapcheck // can't wrap errors in test case
	return t.Port.ConsumeChatRequests(ctx, user, model, inputTokens)
}

func (t *testRateLimiter) ConsumeEmbeddingRequests(
	ctx context.Context, user ids.UserID, model string, tokens int,
) error {
	if err := t.ensurePlan(ctx, user); err != nil {
		return fmt.Errorf("ensuring plan: %w", err)
	}

	//nolint:wrapcheck // can't wrap errors in test case
	return t.Port.ConsumeEmbeddingRequests(ctx, user, model, tokens)
}

func (t *testRateLimiter) ensurePlan(ctx context.Context, user ids.UserID) error {
	const insertUserPlan = `
		INSERT INTO agents.user_plans (user_id, plan_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`
	if _, err := t.pool.Exec(ctx, insertUserPlan, user.ID(), t.planID); err != nil {
		return fmt.Errorf("inserting user plan: %w", err)
	}

	return nil
}

func seeder(pool *pgxpool.Pool) accountsTests.FixtureBuilder {
	return func(ctx context.Context, fixture accountsTests.SaveAccountFixture) error {
		_, err := pool.Exec(ctx, `
				INSERT INTO agents.mcp_servers (id, url)
				VALUES ($1, $2)
				ON CONFLICT DO NOTHING
			`, fixture.AccountID.Server().ID(), "http://test-server")
		if err != nil {
			return fmt.Errorf("inserting server: %w", err)
		}

		_, err = pool.Exec(ctx, `
				INSERT INTO agents.oauth_configs (
					server_id, client_id, client_secret, redirect_url, auth_url, token_url, scopes
				)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
				ON CONFLICT DO NOTHING
			`,
			fixture.AccountID.Server().ID(), "client_id", "client_secret",
			"http://redirect", "http://auth", "http://token",
			[]string{"mcp.read", "mcp.write"},
		)
		if err != nil {
			return fmt.Errorf("inserting oauth config: %w", err)
		}

		return nil
	}
}

func cleaner(pool *pgxpool.Pool) func(context.Context) error {
	return func(ctx context.Context) error {
		tables := []string{
			"agents.agent_settings",
			"agents.mcp_accounts",
			"agents.mcp_servers",
			"agents.oauth_configs",
			"agents.plans",
			"agents.user_plans",
			"agents.rate_limit_buckets",
		}

		for _, table := range tables {
			query := fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table)
			if _, err := pool.Exec(ctx, query); err != nil {
				return fmt.Errorf("truncate %s: %w", table, err)
			}
		}

		return nil
	}
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}

	return v
}

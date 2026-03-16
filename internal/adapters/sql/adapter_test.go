package sql_test

import (
	"context"
	"fmt"
	"net/url"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/testsuite"

	. "github.com/quenbyako/cynosure/internal/adapters/sql"
)

func TestAdapter(t *testing.T) {
	pool := SetupTestDB(t)

	// Get connection string from pool for NewAdapter
	connStr := pool.Config().ConnString()
	adapter, err := New(t.Context(), must(url.Parse(connStr)))
	require.NoError(t, err, "Failed to create SQL adapter")
	require.NotNil(t, adapter, "Adapter should not be nil")

	t.Cleanup(func() {
		if closeErr := adapter.Close(); closeErr != nil {
			t.Errorf("failed to close adapter: %v", closeErr)
		}
	})

	t.Run("Accounts", testsuite.RunAccountStorageTests(adapter,
		testsuite.WithSaveAccountSeeder(seeder(pool)),
		testsuite.WithAccountStorageCleanup(cleaner(pool)),
	))

	t.Run("ModelSettings", testsuite.RunModelSettingsStorageTests(adapter,
		testsuite.WithModelSettingsStorageCleanup(cleaner(pool)),
	))

	t.Run("Servers", testsuite.RunServerStorageTests(adapter,
		testsuite.WithServerStorageCleanup(cleaner(pool)),
	))
}

func seeder(pool *pgxpool.Pool) testsuite.AccountFixtureBuilder {
	return func(ctx context.Context, fixture testsuite.SaveAccountFixture) error {
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

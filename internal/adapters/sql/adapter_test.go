package sql_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/testsuite"

	. "github.com/quenbyako/cynosure/internal/adapters/sql"
)

func TestAdapter(t *testing.T) {
	pool := SetupTestDB(t)

	// Get connection string from pool for NewAdapter
	connStr := pool.Config().ConnString()
	adapter, err := NewAdapter(t.Context(), connStr)
	require.NoError(t, err, "Failed to create SQL adapter")
	require.NotNil(t, adapter, "Adapter should not be nil")
	t.Cleanup(func() { adapter.Close() })

	// Create fixturer function that inserts required servers before account tests
	fixturer := func(fixture testsuite.SaveAccountFixture) error {
		_, err := pool.Exec(t.Context(), "INSERT INTO agents.users (id) VALUES ($1) ON CONFLICT DO NOTHING", fixture.AccountID.User().ID())
		if err != nil {
			return fmt.Errorf("inserting user: %w", err)
		}

		_, err = pool.Exec(t.Context(), `
			INSERT INTO agents.mcp_servers (id, url)
			VALUES ($1, $2)
			ON CONFLICT DO NOTHING
		`, fixture.AccountID.Server().ID(), "http://test-server")
		if err != nil {
			return fmt.Errorf("inserting server: %w", err)
		}

		_, err = pool.Exec(t.Context(), `
			INSERT INTO agents.oauth_configs (server_id, client_id, client_secret, redirect_url, scopes)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT DO NOTHING
		`, fixture.AccountID.Server().ID(), "client_id", "client_secret", "http://redirect", []string{"mcp.read", "mcp.write"})
		if err != nil {
			return fmt.Errorf("inserting oauth config: %w", err)
		}

		return nil
	}

	cleanup := func() error {
		// cleanup flushes the whole data in the test database leaving ONLY
		// schema and roles.
		pool.Exec(t.Context(), "TRUNCATE TABLE agents.model_settings CASCADE")
		pool.Exec(t.Context(), "TRUNCATE TABLE agents.mcp_accounts CASCADE")
		pool.Exec(t.Context(), "TRUNCATE TABLE agents.mcp_servers CASCADE")
		pool.Exec(t.Context(), "TRUNCATE TABLE agents.oauth_configs CASCADE")
		pool.Exec(t.Context(), "TRUNCATE TABLE agents.users CASCADE")

		return nil
	}

	testsuite.RunAccountStorageTests(adapter,
		testsuite.WithSaveAccountSeeder(fixturer),
		testsuite.WithAccountStorageCleanup(cleanup),
	)(t)

	testsuite.RunModelSettingsStorageTests(adapter,
		testsuite.WithModelSettingsStorageCleanup(cleanup),
	)(t)

	testsuite.RunServerStorageTests(adapter,
		testsuite.WithServerStorageCleanup(cleanup),
	)(t)
}

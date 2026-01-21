package sql_test

import (
	"context"
	"fmt"
	"io/fs"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/quenbyako/cynosure/contrib/db"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// SetupTestDB starts a PostgreSQL container, applies schema, and returns a connection pool.
// Automatically registers cleanup with t.Cleanup().
func SetupTestDB(t *testing.T) *pgxpool.Pool {
	dsn, teardown, err := setupDockerPostgres(t)
	require.NoError(t, err, "Failed to start postgres instance")
	t.Cleanup(teardown)

	pool, err := pgxpool.New(t.Context(), dsn)
	require.NoError(t, err, "Failed to create connection pool")
	t.Cleanup(func() { pool.Close() })

	fs.WalkDir(db.Schema, "schema", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		schema, readErr := fs.ReadFile(db.Schema, path)
		if readErr != nil {
			return readErr
		}

		_, execErr := pool.Exec(t.Context(), string(schema))
		if execErr != nil {
			return execErr
		}

		return nil
	})

	return pool
}

// NewTestPostgresInstance starts a PostgreSQL container for testing.
// Returns connection string, teardown function, and error.
// The teardown function should be called via t.Cleanup() or defer.
func setupDockerPostgres(t *testing.T) (string, func(), error) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)

	dbName := "testdb"
	dbUser := "user"
	dbPassword := "password"

	postgresContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase(dbName),
		postgres.WithUsername(dbUser),
		postgres.WithPassword(dbPassword),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		return "", nil, fmt.Errorf("failed to start postgres container: %w", err)
	}

	teardown := func() {
		if err := testcontainers.TerminateContainer(postgresContainer); err != nil {
			t.Logf("failed to stop postgres container: %v", err)
		}
	}

	connStr, err := postgresContainer.ConnectionString(ctx)
	if err != nil {
		teardown()
		return "", nil, fmt.Errorf("failed to get connection string: %w", err)
	}

	return connStr, teardown, nil
}

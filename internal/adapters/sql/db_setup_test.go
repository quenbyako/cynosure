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
	t.Helper()

	dsn, teardown, err := setupDockerPostgres(t)
	require.NoError(t, err, "Failed to start postgres instance")
	t.Cleanup(teardown)

	pool, err := pgxpool.New(t.Context(), dsn)
	require.NoError(t, err, "Failed to create connection pool")
	t.Cleanup(func() { pool.Close() })

	err = fs.WalkDir(db.Schema, "schema", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		schema, readErr := fs.ReadFile(db.Schema, path)
		if readErr != nil {
			return fmt.Errorf("reading schema file %s: %w", path, readErr)
		}

		_, execErr := pool.Exec(t.Context(), string(schema))
		if execErr != nil {
			return fmt.Errorf("executing schema %s: %w", path, execErr)
		}

		return nil
	})
	require.NoError(t, err, "Failed to apply database schema")

	return pool
}

// NewTestPostgresInstance starts a PostgreSQL container for testing.
// Returns connection string, teardown function, and error.
// The teardown function should be called via t.Cleanup() or defer.
func setupDockerPostgres(t *testing.T) (dsn string, teardown func(), err error) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)

	dbName := "testdb"
	dbUser := "user"
	dbPassword := "password"

	postgresContainer, execErr := postgres.Run(ctx,
		"pgvector/pgvector:pg16",
		postgres.WithDatabase(dbName),
		postgres.WithUsername(dbUser),
		postgres.WithPassword(dbPassword),
		postgres.BasicWaitStrategies(),
	)
	if execErr != nil {
		return "", nil, fmt.Errorf("failed to start postgres container: %w", execErr)
	}

	teardown = func() {
		if err := testcontainers.TerminateContainer(postgresContainer); err != nil {
			t.Logf("failed to stop postgres container: %v", err)
		}
	}

	connStr, connErr := postgresContainer.ConnectionString(ctx)
	if connErr != nil {
		teardown()
		return "", nil, fmt.Errorf("failed to get connection string: %w", connErr)
	}

	return connStr, teardown, nil
}

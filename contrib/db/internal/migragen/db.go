package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func createDatabases(
	ctx context.Context,
	connStr string,
) (current, target string, err error) {
	database, err := sql.Open("pgx", connStr)
	if err != nil {
		return "", "", fmt.Errorf("opening master db: %w", err)
	}

	defer func() {
		errClose := database.Close()
		if errClose != nil {
			_ = errClose
		}
	}()

	if _, err := database.ExecContext(ctx, "CREATE DATABASE db_current"); err != nil {
		return "", "", fmt.Errorf("creating db_current: %w", err)
	}

	if _, err := database.ExecContext(ctx, "CREATE DATABASE db_target"); err != nil {
		return "", "", fmt.Errorf("creating db_target: %w", err)
	}

	return replaceDBName(connStr, "db_current"), replaceDBName(connStr, "db_target"), nil
}

func replaceDBName(connStr, dbName string) string {
	parsedURL, err := url.Parse(connStr)
	if err != nil {
		return ""
	}

	parsedURL.Path = "/" + dbName

	return parsedURL.String()
}

func applyMigrations(
	ctx context.Context,
	dbCurrent *sql.DB,
	dir string,
) error {
	files, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading migrations directory: %w", err)
	}

	var upFiles []string

	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".up.sql") {
			upFiles = append(upFiles, f.Name())
		}
	}

	slices.Sort(upFiles)

	for _, filename := range upFiles {
		if err := applyMigrationFile(ctx, dbCurrent, filepath.Join(dir, filename)); err != nil {
			return err
		}
	}

	return nil
}

func applyMigrationFile(ctx context.Context, dbCurrent *sql.DB, path string) error {
	content, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("reading migration file %s: %w", path, err)
	}

	statements := splitSQLStatements(string(content))
	for _, stmt := range statements {
		if _, err := dbCurrent.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("executing statement in %s: %w\nStatement: %s", path, err, stmt)
		}
	}

	return nil
}

func applySchema(ctx context.Context, dbTarget *sql.DB, schemaFile string) error {
	content, err := os.ReadFile(filepath.Clean(schemaFile))
	if err != nil {
		return fmt.Errorf("reading schema file %s: %w", schemaFile, err)
	}

	statements := splitSQLStatements(string(content))
	for _, stmt := range statements {
		if _, err := dbTarget.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf(
				"applying statement in %s: %w\nStatement: %s",
				schemaFile,
				err,
				stmt,
			)
		}
	}

	return nil
}

func openCurrentAndTargetDBs(
	currentDSN, targetDSN string,
) (currentDB, targetDB *sql.DB, err error) {
	dbCurrent, err := sql.Open("pgx", currentDSN)
	if err != nil {
		return nil, nil, fmt.Errorf("opening current db: %w", err)
	}

	dbTarget, err := sql.Open("pgx", targetDSN)
	if err != nil {
		errClose := dbCurrent.Close()
		if errClose != nil {
			_ = errClose
		}

		return nil, nil, fmt.Errorf("opening target db: %w", err)
	}

	return dbCurrent, dbTarget, nil
}

func closeDatabases(dbCurrent, dbTarget *sql.DB) {
	if err := dbCurrent.Close(); err != nil {
		_ = err
	}

	if err := dbTarget.Close(); err != nil {
		_ = err
	}
}

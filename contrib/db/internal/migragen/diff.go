package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/stripe/pg-schema-diff/pkg/diff"
	"github.com/stripe/pg-schema-diff/pkg/tempdb"
)

func runGenerator(
	ctx context.Context,
	dir string,
	schema string,
	nextNum int,
	name string,
) error {
	pgCont, err := startPostgresContainer(ctx)
	if err != nil {
		return fmt.Errorf("starting postgres container: %w", err)
	}

	defer pgCont.teardown()

	return runGeneratorWithContainer(
		ctx,
		pgCont.connStr,
		dir,
		schema,
		nextNum,
		name,
	)
}

func runGeneratorWithContainer(
	ctx context.Context,
	connStr string,
	dir string,
	schema string,
	nextNum int,
	name string,
) error {
	currentDSN, targetDSN, err := createDatabases(ctx, connStr)
	if err != nil {
		return err
	}

	dbCurrent, dbTarget, err := openCurrentAndTargetDBs(currentDSN, targetDSN)
	if err != nil {
		return err
	}

	defer closeDatabases(dbCurrent, dbTarget)

	return executeMigrationAndGeneration(
		ctx,
		dbCurrent,
		dbTarget,
		connStr,
		nextNum,
		name,
		dir,
		schema,
	)
}

func executeMigrationAndGeneration(
	ctx context.Context,
	dbCurrent *sql.DB,
	dbTarget *sql.DB,
	connStr string,
	nextNum int,
	name string,
	dir string,
	schema string,
) error {
	if err := applyMigrations(ctx, dbCurrent, dir); err != nil {
		return err
	}

	if err := applySchema(ctx, dbTarget, schema); err != nil {
		return err
	}

	return generateAndSaveMigration(
		ctx,
		dbCurrent,
		dbTarget,
		connStr,
		nextNum,
		name,
		dir,
	)
}

func generateAndSaveMigration(
	ctx context.Context,
	dbCurrent *sql.DB,
	dbTarget *sql.DB,
	connStr string,
	nextNum int,
	name string,
	dir string,
) error {
	tempDBFactory, err := createTempDBFactory(ctx, connStr)
	if err != nil {
		return err
	}
	defer closeTempDBFactory(tempDBFactory)

	opts := []diff.PlanOpt{
		diff.WithTempDbFactory(tempDBFactory),
		diff.WithIncludeSchemas("agents"),
	}

	return planAndSave(ctx, dbCurrent, dbTarget, opts, nextNum, name, dir)
}

func planAndSave(
	ctx context.Context,
	dbCurrent *sql.DB,
	dbTarget *sql.DB,
	opts []diff.PlanOpt,
	nextNum int,
	name string,
	dir string,
) error {
	planUp, err := generateUpPlan(ctx, dbCurrent, dbTarget, opts)
	if err != nil {
		return err
	}

	planDown, err := generateDownPlan(ctx, dbCurrent, dbTarget, opts)
	if err != nil {
		return err
	}

	return writePlanFiles(planUp, planDown, dir, nextNum, name)
}

func closeTempDBFactory(factory tempdb.Factory) {
	if err := factory.Close(); err != nil {
		_ = err
	}
}

func generateUpPlan(
	ctx context.Context,
	dbCurrent *sql.DB,
	dbTarget *sql.DB,
	opts []diff.PlanOpt,
) (diff.Plan, error) {
	plan, err := diff.Generate(
		ctx,
		diff.DBSchemaSource(dbCurrent),
		diff.DBSchemaSource(dbTarget),
		opts...,
	)
	if err != nil {
		return diff.Plan{}, fmt.Errorf("generating up plan: %w", err)
	}

	return plan, nil
}

func generateDownPlan(
	ctx context.Context,
	dbCurrent *sql.DB,
	dbTarget *sql.DB,
	opts []diff.PlanOpt,
) (diff.Plan, error) {
	plan, err := diff.Generate(
		ctx,
		diff.DBSchemaSource(dbTarget),
		diff.DBSchemaSource(dbCurrent),
		opts...,
	)
	if err != nil {
		return diff.Plan{}, fmt.Errorf("generating down plan: %w", err)
	}

	return plan, nil
}

//nolint:ireturn // tempdb.Factory is interface
func createTempDBFactory(
	ctx context.Context,
	connStr string,
) (tempdb.Factory, error) {
	factory, err := tempdb.NewOnInstanceFactory(
		ctx,
		func(ctx context.Context, dbName string) (*sql.DB, error) {
			tempDB, err := sql.Open("pgx", replaceDBName(connStr, dbName))
			if err != nil {
				return nil, fmt.Errorf("opening temp db: %w", err)
			}

			if err := setupTempDB(ctx, tempDB); err != nil {
				if errClose := tempDB.Close(); errClose != nil {
					_ = errClose
				}

				return nil, fmt.Errorf("setting up temp db: %w", err)
			}

			return tempDB, nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("creating temp db factory: %w", err)
	}

	return factory, nil
}

func setupTempDB(ctx context.Context, database *sql.DB) error {
	_, err := database.ExecContext(ctx, `
		CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
		CREATE EXTENSION IF NOT EXISTS "vector";
	`)
	if err != nil {
		return fmt.Errorf("installing extensions in temp db: %w", err)
	}

	return nil
}

func writePlanFiles(
	planUp diff.Plan,
	planDown diff.Plan,
	dir string,
	nextNum int,
	name string,
) error {
	if len(planUp.Statements) == 0 && len(planDown.Statements) == 0 {
		fmt.Printf("No schema changes detected.\n")

		return nil
	}

	prefix := filepath.Join(dir, fmt.Sprintf("%06d_%s", nextNum, name))

	err := saveMigrationFile(prefix+".up.sql", formatPlanSQL(planUp))
	if err != nil {
		return err
	}

	err = saveMigrationFile(prefix+".down.sql", formatPlanSQL(planDown))
	if err != nil {
		return err
	}

	fmt.Printf("Generated migrations:\n  %s.up.sql\n  %s.down.sql\n", prefix, prefix)

	return nil
}

func saveMigrationFile(path, content string) error {
	const writePerm = 0o600
	if err := os.WriteFile(path, []byte(content), writePerm); err != nil {
		return fmt.Errorf("writing migration %s: %w", path, err)
	}

	return nil
}

func formatPlanSQL(plan diff.Plan) string {
	var builder strings.Builder

	for _, stmt := range plan.Statements {
		for _, hazard := range stmt.Hazards {
			//nolint:forbidigo // warning output
			fmt.Fprintf(
				os.Stderr,
				"WARNING: Hazard detected [%s]: %s\n",
				hazard.Type,
				hazard.Message,
			)
		}

		query := stmt.DDL
		if !strings.HasSuffix(query, ";") {
			query += ";"
		}

		builder.WriteString(query)
		builder.WriteString("\n\n")
	}

	return builder.String()
}

func getNextMigrationNumber(dir string) (int, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 1, nil
		}

		return 0, fmt.Errorf("reading migration dir: %w", err)
	}

	maxNum, err := findMaxMigrationNumber(files)
	if err != nil {
		return 0, err
	}

	return maxNum + 1, nil
}

func findMaxMigrationNumber(files []os.DirEntry) (int, error) {
	maxNum := 0
	matcher := regexp.MustCompile(`^(\d+)_.*\.up\.sql$`)

	for _, f := range files {
		if f.IsDir() {
			continue
		}

		num, err := parseMigrationNumber(f.Name(), matcher)
		if err != nil {
			return 0, err
		}

		if num > maxNum {
			maxNum = num
		}
	}

	return maxNum, nil
}

func parseMigrationNumber(filename string, matcher *regexp.Regexp) (int, error) {
	const expectedMatches = 2

	matches := matcher.FindStringSubmatch(filename)
	if len(matches) != expectedMatches {
		return 0, nil
	}

	num, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, fmt.Errorf("parsing migration prefix %s: %w", matches[1], err)
	}

	return num, nil
}

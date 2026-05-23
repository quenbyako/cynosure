// Package main implements the CLI tool for generating DB migrations.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"
)

const (
	defaultTimeout = 2 * time.Minute
	imageName      = "pgvector/pgvector:pg16"
)

func main() {
	name := flag.String("name", "auto", "Name of the migration")
	schema := flag.String("schema", "./schema/schema.sql", "Path to schema.sql")
	dir := flag.String("dir", "./migrations", "Migrations directory")

	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	if err := run(ctx, *dir, *schema, *name); err != nil {
		//nolint:forbidigo // CLI error print
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)

		//nolint:forbidigo // CLI exit
		os.Exit(1)
	}
}

func run(ctx context.Context, dir, schema, name string) error {
	nextNum, err := getNextMigrationNumber(dir)
	if err != nil {
		return fmt.Errorf("getting next migration number: %w", err)
	}

	if err = runGenerator(ctx, dir, schema, nextNum, name); err != nil {
		return fmt.Errorf("running generator: %w", err)
	}

	return nil
}

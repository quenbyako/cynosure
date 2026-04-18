// Package db provides schema-based client to database.
package db

import (
	"embed"
)

//go:embed schema
var Schema embed.FS

//go:generate go run github.com/sqlc-dev/sqlc/cmd/sqlc@latest generate

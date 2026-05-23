// Package accounts defines the port interfaces and data structures for managing Cynosure accounts.
package accounts

import (
	"errors"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
)

var (
	ErrNotFound         = ports.ErrNotFound
	ErrInvalidAccountID = errors.New("invalid account ID")
)

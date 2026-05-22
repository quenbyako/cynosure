// Package accounts defines the port interfaces and data structures for managing Cynosure accounts.
package accounts

import (
	"errors"
)

var (
	ErrNotFound         = errors.New("not found")
	ErrInvalidAccountID = errors.New("invalid account ID")
)

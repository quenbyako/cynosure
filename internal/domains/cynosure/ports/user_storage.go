package ports

import (
	"context"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

// UserStorage manages user existence checks.
// Quick fix for FK constraints until a proper User aggregate is implemented.
type UserStorage interface {
	// HasUser returns true if the user exists in the system.
	//
	// TODO: Replace with GetUser when User entities are implemented.
	HasUser(ctx context.Context, id ids.UserID) (bool, error)

	// LookupUser returns the user ID associated with the given provider and
	// external ID.
	//
	// Throws:
	//   - [ErrNotFound]: if no such identity exists.
	LookupUser(ctx context.Context, provider, externalID string) (ids.UserID, error)

	// NewUserWithIdentity creates a new user and associates it with the given
	// provider and external ID.
	//
	// Throws:
	//   - [ErrAlreadyExists]: if the user already exists.
	NewUserWithIdentity(ctx context.Context, id ids.UserID, provider, externalID string) error
}

type UserStorageFactory interface {
	UserStorage() UserStorage
}

func NewUserStorage(factory UserStorageFactory) UserStorage { return factory.UserStorage() }

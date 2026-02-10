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
}

type UserStorageFactory interface {
	UserStorage() UserStorage
}

func NewUserStorage(factory UserStorageFactory) UserStorage { return factory.UserStorage() }

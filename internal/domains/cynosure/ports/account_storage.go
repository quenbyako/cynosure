package ports

import (
	"context"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/ids"
)

// AccountStorage is responsible for storing and retrieving MCP account
// information. This port IS NOT RELATED to user accounts, those do not exist
// in current domain.
type AccountStorage interface {
	AccountStorageRead
	AccountStorageWrite
}

type AccountStorageFactory interface {
	AccountStorage() AccountStorage
}

func NewAccountStorage(factory AccountStorageFactory) AccountStorage { return factory.AccountStorage() }

type AccountStorageRead interface {
	ListAccounts(ctx context.Context, user ids.UserID) ([]ids.AccountID, error)
	GetAccount(ctx context.Context, account ids.AccountID) (*entities.Account, error)
	GetAccountsBatch(ctx context.Context, accounts []ids.AccountID) ([]*entities.Account, error)
}

type AccountStorageWrite interface {
	SaveAccount(ctx context.Context, info entities.AccountReadOnly) error
	DeleteAccount(ctx context.Context, account ids.AccountID) error
}

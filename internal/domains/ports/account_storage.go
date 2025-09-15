package ports

import (
	"context"

	"tg-helper/internal/domains/components/ids"
	"tg-helper/internal/domains/entities"
)

type AccountStorage interface {
	AccountStorageRead
	AccountStorageWrite
}

type AccountStorageRead interface {
	ListAccounts(ctx context.Context, user ids.UserID) ([]ids.AccountID, error)
	GetAccount(ctx context.Context, account ids.AccountID) (*entities.Account, error)
	GetAccountsBatch(ctx context.Context, accounts []ids.AccountID) ([]*entities.Account, error)
}

type AccountStorageWrite interface {
	SaveAccount(ctx context.Context, info entities.AccountReadOnly) error
	DeleteAccount(ctx context.Context, account ids.AccountID) error
}

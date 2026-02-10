package ports

import (
	"context"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

// AccountStorage manages persistence of MCP account aggregates.
// Note: This is NOT related to user accounts - it stores MCP server connections
// with their OAuth tokens, tools, and metadata.
//
// Accounts are the primary way users interact with external MCP servers.
// Each account represents an authenticated connection to one server.
type AccountStorage interface {
	AccountStorageRead
	AccountStorageWrite
}

type AccountStorageFactory interface {
	AccountStorage() AccountStorage
}

func NewAccountStorage(factory AccountStorageFactory) AccountStorage { return factory.AccountStorage() }

type AccountStorageRead interface {
	// ListAccounts returns all account IDs owned by the user. Empty slice if
	// none exist.
	//
	// See next test suites to find how it works:
	//
	//  -  [TestSaveAccount] — checking that account is retrievable after saving
	ListAccounts(ctx context.Context, user ids.UserID) ([]ids.AccountID, error)

	// GetAccount retrieves full account info.
	//
	// See next test suites to find how it works:
	//
	// 	- [TestSaveAccount] — checking that specific account is retrievable
	//    after saving
	//
	// Throws:
	//
	//  - [ErrNotFound] if account doesn't exist.
	GetAccount(ctx context.Context, account ids.AccountID) (*entities.Account, error)

	// GetAccountsBatch fetches multiple accounts in one operation. Silently
	// skips non-existent accounts, returning partial results in arbitrary
	// order.
	//
	// See next test suites to find how it works:
	//
	//  -  [TestSaveAccount] — batch retrieval of stored accounts
	GetAccountsBatch(ctx context.Context, accounts []ids.AccountID) ([]*entities.Account, error)
}

type AccountStorageWrite interface {
	// SaveAccount creates or updates account (upsert). Atomically replaces
	// associated tools. Does not validate server existence or tool schemas -
	// validation happens in domain layer.
	//
	// See next test suites to find how it works:
	//
	//  -  [TestSaveAccount] — persisting account with tools and retrieving it
	//     back
	SaveAccount(ctx context.Context, info entities.AccountReadOnly) error

	// DeleteAccount soft-deletes account (marks as deleted, keeps data on
	// disk). Idempotent operation. Does not cascade to related entities like
	// threads or messages.
	//
	// See next test suites to find how it works:
	//
	//  -  (tested in integration tests, no dedicated suite yet)
	DeleteAccount(ctx context.Context, account ids.AccountID) error
}

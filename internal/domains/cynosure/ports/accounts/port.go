package accounts

import (
	"context"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

type PortFactory interface {
	Accounts() PortWrapped
}

// Port manages persistence of MCP account aggregates.
// Note: This is NOT related to user accounts - it stores MCP server connections
// with their OAuth tokens, tools, and metadata.
//
// Accounts are the primary way users interact with external MCP servers.
// Each account represents an authenticated connection to one server.
type Port interface {
	PortRead
	PortWrite
}

type PortRead interface {
	// ListAccounts returns all account IDs owned by the user. Empty slice if
	// none exist.
	//
	// See next test suites to find how it works:
	//
	//  -  [TestSaveAccount] — checking that account is retrievable after saving
	ListAccounts(ctx context.Context, user ids.UserID) ([]ids.AccountID, error)

	// GetAccount retrieves full account info.
	//
	// Throws:
	//
	//  - [ErrNotFound] if account doesn't exist.
	GetAccount(
		ctx context.Context,
		account ids.AccountID,
		opts ...GetAccountOption,
	) (*entities.Account, error)

	// GetAccountsBatch fetches multiple accounts in one operation. Silently
	// skips non-existent accounts, returning partial results in arbitrary
	// order.
	GetAccountsBatch(ctx context.Context, accounts []ids.AccountID) ([]*entities.Account, error)

	// FindAccountsByName retrieves all accounts (active and deleted) matching
	// the name for the user.
	FindAccountsByName(ctx context.Context, user ids.UserID, name string) ([]SearchResult, error)
}

type PortWrite interface {
	// SaveAccount creates or updates account (upsert). Atomically replaces
	// associated tools. Does not validate server existence or tool schemas -
	// validation happens in domain layer.
	SaveAccount(ctx context.Context, info entities.AccountReadOnly) error

	// DeleteAccount soft-deletes account (marks as deleted, keeps data on
	// disk). Idempotent operation. Does not cascade to related entities like
	// threads or messages.
	DeleteAccount(ctx context.Context, account ids.AccountID) error

	// ReactivateAccount marks a previously soft-deleted account (and its tools)
	// as active.
	ReactivateAccount(ctx context.Context, account ids.AccountID) error
}

func defaultGetAccountParams(required getAccountRequiredParams) getAccountParams {
	return getAccountParams{
		getAccountRequiredParams: required,
		includeDeleted:           false,
	}
}

func (s *getAccountParams) validate() error {
	if !s.account.Valid() {
		return ErrInvalidAccountID
	}

	return nil
}

// SearchResult represents a unified search result for an account.
//
// Implemented by these types:
//
//   - [SearchResultActive]
//   - [SearchResultDeleted]
type SearchResult interface {
	ID() ids.AccountID
	_SearchResult()
}

// SearchResultActive wraps a live, active account.
type SearchResultActive struct {
	Account *entities.Account
}

func (SearchResultActive) _SearchResult() {}

func (r SearchResultActive) ID() ids.AccountID { return r.Account.ID() }

// SearchResultDeleted wraps a soft-deleted account identity.
type SearchResultDeleted struct {
	AccountID ids.AccountID
}

func (SearchResultDeleted) _SearchResult() {}

func (r SearchResultDeleted) ID() ids.AccountID { return r.AccountID }

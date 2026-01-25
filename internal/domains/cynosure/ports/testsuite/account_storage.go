package testsuite

import (
	"errors"
	"testing"

	suites "github.com/quenbyako/cynosure/contrib/bettersuites"
	"github.com/stretchr/testify/require"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

// RunAccountStorageTests runs tests for the given adapter. These tests are predefined
// and REQUIRED to be used for ANY adapter implementation.
//
// After constructing test function, just run it through `run(t)` call,
// everything else will be handled for you. Calling this function through
// `t.Run("general", run)` is not very recommended, cause test logs will be too
// hard to read cause of big nesting.
func RunAccountStorageTests(a ports.AccountStorage, opts ...AccountStorageTestSuiteOption) func(t *testing.T) {
	s := &AccountStorageTestSuite{
		adapter: a,
	}
	for _, opt := range opts {
		opt(s)
	}
	if err := s.validate(); err != nil {
		panic(err)
	}

	return suites.Run(s)
}

type AccountStorageTestSuite struct {
	adapter ports.AccountStorage

	saveAccountFixture func(SaveAccountFixture) error
	cleanup            func() error
}

var _ suites.AfterTest = (*AccountStorageTestSuite)(nil)

type AccountStorageTestSuiteOption func(*AccountStorageTestSuite)

func WithSaveAccountSeeder(f func(SaveAccountFixture) error) AccountStorageTestSuiteOption {
	return func(s *AccountStorageTestSuite) { s.saveAccountFixture = f }
}

func WithAccountStorageCleanup(f func() error) AccountStorageTestSuiteOption {
	return func(s *AccountStorageTestSuite) { s.cleanup = f }
}

func (s *AccountStorageTestSuite) validate() error {
	if s.adapter == nil {
		return errors.New("adapter is nil")
	}

	return nil
}

func (s *AccountStorageTestSuite) AfterTest(t *testing.T) {
	if s.cleanup != nil {
		if err := s.cleanup(); err != nil {
			t.Fatalf("cleanup failed: %v", err)
		}
	}
}

type SaveAccountFixture struct {
	AccountID   ids.AccountID
	Name        string
	Description string
}

func (s *AccountStorageTestSuite) TestSaveAccount(t *testing.T) {
	fixture := s.buildSaveAccountSeed()

	if s.saveAccountFixture != nil {
		if err := s.saveAccountFixture(fixture); err != nil {
			t.Fatalf("failed to setup fixtures: %v", err)
		}
	}

	account := must(entities.NewAccount(
		fixture.AccountID,
		fixture.Name,
		fixture.Description,
	))

	t.Run("saving_account", func(t *testing.T) {
		err := s.adapter.SaveAccount(t.Context(), account)
		require.NoError(t, err, "failed to save account")
	})

	t.Run("retrieving_account", func(t *testing.T) {
		got, err := s.adapter.GetAccount(t.Context(), fixture.AccountID)
		require.NoError(t, err, "failed to get account")
		require.NotNil(t, got, "retrieved account is nil")
		require.Equal(t, account.ID(), got.ID(), "account ID mismatch")
		require.Equal(t, account.Name(), got.Name(), "account name mismatch")
		require.Equal(t, account.Description(), got.Description(), "account description mismatch")
	})

	t.Run("listing_accounts", func(t *testing.T) {
		ids, err := s.adapter.ListAccounts(t.Context(), fixture.AccountID.User())
		require.NoError(t, err, "failed to list accounts")
		require.Len(t, ids, 1, "adapter should return exactly one account")
		require.Contains(t, ids, fixture.AccountID, "account ID not found in list")
	})
}

func (s *AccountStorageTestSuite) buildSaveAccountSeed() SaveAccountFixture {
	serverID := ids.RandomServerID()
	userID := ids.RandomUserID()

	account := must(ids.RandomAccountID(userID, serverID))

	return SaveAccountFixture{
		AccountID:   account,
		Name:        "Test Account",
		Description: "Some description",
	}
}

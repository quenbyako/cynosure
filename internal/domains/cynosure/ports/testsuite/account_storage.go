package testsuite

import (
	"context"
	"errors"
	"testing"

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
func RunAccountStorageTests(
	a ports.AccountStorage, opts ...AccountStorageTestSuiteOption,
) func(t *testing.T) {
	suite := &AccountStorageTestSuite{
		adapter:            a,
		saveAccountFixture: nil,
		cleanup:            nil,
	}
	for _, opt := range opts {
		opt(suite)
	}

	if err := suite.validate(); err != nil {
		panic(err) //nolint:forbidigo // ok for tests
	}

	return runSuite(suite)
}

type AccountStorageTestSuite struct {
	adapter ports.AccountStorage

	saveAccountFixture AccountFixtureBuilder
	cleanup            func(context.Context) error
}

var _ afterTest = (*AccountStorageTestSuite)(nil)

type AccountStorageTestSuiteOption func(*AccountStorageTestSuite)

type AccountFixtureBuilder = func(context.Context, SaveAccountFixture) error

func WithSaveAccountSeeder(f AccountFixtureBuilder) AccountStorageTestSuiteOption {
	return func(s *AccountStorageTestSuite) { s.saveAccountFixture = f }
}

func WithAccountStorageCleanup(f func(context.Context) error) AccountStorageTestSuiteOption {
	return func(s *AccountStorageTestSuite) { s.cleanup = f }
}

func (s *AccountStorageTestSuite) validate() error {
	if s.adapter == nil {
		return errors.New("adapter is nil") //nolint:err113 // ok for tests
	}

	return nil
}

func (s *AccountStorageTestSuite) afterTest(t *testing.T) {
	t.Helper()

	t.Helper()

	if s.cleanup != nil {
		if err := s.cleanup(t.Context()); err != nil {
			t.Fatalf("cleanup failed: %v", err)
		}
	}
}

type SaveAccountFixture struct {
	Name        string
	Description string
	AccountID   ids.AccountID
}

func (s *AccountStorageTestSuite) TestSaveAccount(t *testing.T) {
	fixture, account := s.setupSaveAccountTest(t)

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
		accountIDs, err := s.adapter.ListAccounts(t.Context(), fixture.AccountID.User())
		require.NoError(t, err, "failed to list accounts")
		require.Len(t, accountIDs, 1, "adapter should return exactly one account")
		require.Contains(t, accountIDs, fixture.AccountID, "account ID not found in list")
	})
}

func (s *AccountStorageTestSuite) setupSaveAccountTest(
	t *testing.T,
) (SaveAccountFixture, *entities.Account) {
	t.Helper()

	fixture := s.buildSaveAccountSeed()

	if s.saveAccountFixture != nil {
		if err := s.saveAccountFixture(t.Context(), fixture); err != nil {
			t.Fatalf("failed to setup fixtures: %v", err)
		}
	}

	account := must(entities.NewAccount(fixture.AccountID, fixture.Name, fixture.Description))

	return fixture, account
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

func must[T any](v T, err error) T {
	if err != nil {
		panic(err) //nolint:forbidigo
	}

	return v
}

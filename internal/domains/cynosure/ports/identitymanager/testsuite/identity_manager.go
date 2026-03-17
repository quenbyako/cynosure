package testsuite

import (
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/identitymanager"
)

// RunIdentityManagerTests runs tests for the given adapter. These tests are
// predefined and REQUIRED to be used for ANY adapter implementation.
func RunIdentityManagerTests(
	a identitymanager.Port, opts ...IdentityManagerTestSuiteOption,
) func(t *testing.T) {
	suite := &IdentityManagerTestSuite{
		adapter: a,
		cleanup: nil,
	}
	for _, opt := range opts {
		opt(suite)
	}

	if err := suite.validate(); err != nil {
		panic(err) //nolint:forbidigo // ok for tests
	}

	return runSuite(suite)
}

type IdentityManagerTestSuite struct {
	adapter identitymanager.Port

	cleanup func() error
}

var _ afterTest = (*IdentityManagerTestSuite)(nil)

type IdentityManagerTestSuiteOption func(*IdentityManagerTestSuite)

func WithIdentityManagerCleanup(f func() error) IdentityManagerTestSuiteOption {
	return func(s *IdentityManagerTestSuite) { s.cleanup = f }
}

func (s *IdentityManagerTestSuite) validate() error {
	if s.adapter == nil {
		return errors.New("adapter is nil")
	}

	return nil
}

func (s *IdentityManagerTestSuite) afterTest(t *testing.T) {
	t.Helper()

	if s.cleanup != nil {
		if err := s.cleanup(); err != nil {
			t.Fatalf("cleanup failed: %v", err)
		}
	}
}

func (s *IdentityManagerTestSuite) TestIdentityFlow(t *testing.T) {
	id := time.Now().UnixNano() / int64(time.Microsecond)

	externalID := strconv.FormatInt(id, 10)
	nickname := fmt.Sprintf("go-test-user-%d", id)
	firstName := "Test"
	lastName := "User"

	t.Run("create_identity", func(t *testing.T) {
		userID, err := s.adapter.CreateUser(t.Context(), externalID, nickname, firstName, lastName)
		require.NoError(t, err)
		require.True(t, userID.Valid())

		t.Run("has_user", func(t *testing.T) {
			has, err := s.adapter.HasUser(t.Context(), userID)
			require.NoError(t, err)
			require.True(t, has)
		})

		t.Run("lookup_user", func(t *testing.T) {
			gotID, err := s.adapter.LookupUser(t.Context(), externalID)
			require.NoError(t, err)
			require.Equal(t, userID, gotID)
		})

		t.Run("issue_token", func(t *testing.T) {
			token, err := s.adapter.IssueToken(t.Context(), userID)
			require.NoError(t, err)
			require.NotNil(t, token)
			require.NotEmpty(t, token.AccessToken)
		})
	})

	t.Run("not_found", func(t *testing.T) {
		_, err := s.adapter.LookupUser(t.Context(), "nonexistent")
		require.ErrorIs(t, err, identitymanager.ErrNotFound)
	})
}

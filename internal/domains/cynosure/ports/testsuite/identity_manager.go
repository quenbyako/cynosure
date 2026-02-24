package testsuite

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
)

// RunIdentityManagerTests runs tests for the given adapter. These tests are
// predefined and REQUIRED to be used for ANY adapter implementation.
func RunIdentityManagerTests(a ports.IdentityManager, opts ...IdentityManagerTestSuiteOption) func(t *testing.T) {
	s := &IdentityManagerTestSuite{
		adapter: a,
	}
	for _, opt := range opts {
		opt(s)
	}
	if err := s.validate(); err != nil {
		panic(err)
	}

	return runSuite(s)
}

type IdentityManagerTestSuite struct {
	adapter ports.IdentityManager

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
	if s.cleanup != nil {
		if err := s.cleanup(); err != nil {
			t.Fatalf("cleanup failed: %v", err)
		}
	}
}

func (s *IdentityManagerTestSuite) TestIdentityFlow(t *testing.T) {
	externalID := "ext-123"
	nickname := "tester"
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

		t.Run("save_mapping", func(t *testing.T) {
			// Test that we can save another mapping for same user
			err := s.adapter.SaveUserMapping(t.Context(), userID, "another", "id-456")
			require.NoError(t, err)

			gotID, err := s.adapter.LookupUser(t.Context(), "id-456")
			require.NoError(t, err)
			require.Equal(t, userID, gotID)
		})
	})

	t.Run("not_found", func(t *testing.T) {
		_, err := s.adapter.LookupUser(t.Context(), "nonexistent")
		require.ErrorIs(t, err, ports.ErrNotFound)
	})
}

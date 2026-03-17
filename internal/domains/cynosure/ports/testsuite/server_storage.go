package testsuite

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

// RunServerStorageTests runs tests for the given adapter. These tests are predefined
// and REQUIRED to be used for ANY adapter implementation.
//
// After constructing test function, just run it through `run(t)` call,
// everything else will be handled for you. Calling this function through
// `t.Run("general", run)` is not very recommended, cause test logs will be too
// hard to read cause of big nesting.
func RunServerStorageTests(
	a ports.ServerStorage, opts ...ServerStorageTestSuiteOption,
) func(t *testing.T) {
	suite := &ServerStorageTestSuite{
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

type ServerStorageTestSuite struct {
	adapter ports.ServerStorage

	cleanup func(context.Context) error
}

var _ afterTest = (*ServerStorageTestSuite)(nil)

type ServerStorageTestSuiteOption func(*ServerStorageTestSuite)

func WithServerStorageCleanup(f func(context.Context) error) ServerStorageTestSuiteOption {
	return func(s *ServerStorageTestSuite) { s.cleanup = f }
}

func (s *ServerStorageTestSuite) validate() error {
	if s.adapter == nil {
		return errors.New("adapter is nil")
	}

	return nil
}

func (s *ServerStorageTestSuite) afterTest(t *testing.T) {
	t.Helper()

	if s.cleanup != nil {
		if err := s.cleanup(t.Context()); err != nil {
			t.Fatalf("cleanup failed: %v", err)
		}
	}
}

const (
	oneDay = 24 * time.Hour
)

func (s *ServerStorageTestSuite) TestSaveServer(t *testing.T) {
	serverID := ids.RandomServerID()
	link := must(url.Parse("https://example.com/sse"))
	opts := []entities.ServerConfigOption{
		entities.WithExpiration(time.Now().Add(oneDay)),
		entities.WithAuthConfig(&oauth2.Config{
			ClientID: "client-id",
			Endpoint: oauth2.Endpoint{ //nolint:gosec // not a credential
				AuthURL:       "https://example.com/auth",
				TokenURL:      "https://example.com/token",
				DeviceAuthURL: "",
				AuthStyle:     0,
			},
			ClientSecret: "",
			RedirectURL:  "",
			Scopes:       nil,
		}),
	}
	server := must(entities.NewServerConfig(serverID, link, opts...))

	t.Run("saving_server", func(t *testing.T) {
		err := s.adapter.SetServer(t.Context(), server)
		require.NoError(t, err, "failed to save server")
	})

	t.Run("retrieving_server", func(t *testing.T) {
		got, err := s.adapter.GetServerInfo(t.Context(), serverID)
		require.NoError(t, err, "failed to get server")
		require.NotNil(t, got, "retrieved server is nil")
		require.Equal(t, server.ID(), got.ID(), "server ID mismatch")
	})

	t.Run("listing_servers", func(t *testing.T) {
		servers, err := s.adapter.ListServers(t.Context())
		require.NoError(t, err, "failed to list servers")
		require.Len(t, servers, 1, "adapter should return exactly one server")
		require.Equal(t, serverID, servers[0].ID(), "server ID not found in list")
	})
}

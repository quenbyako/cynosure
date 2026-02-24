package mcp_test

import (
	"context"
	"net/url"
	"testing"

	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/testsuite"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"

	. "github.com/quenbyako/cynosure/internal/adapters/mcp"
)

func TestToolClientSuite(t *testing.T) {
	var (
		serverConfig  entities.ServerConfigReadOnly
		accountTokens map[ids.AccountID]*oauth2.Token
	)

	// SaveTokenFunc mock
	storage := func(ctx context.Context, account ids.AccountID, t *oauth2.Token) error {
		accountTokens[account] = t
		return nil
	}

	// AccountTokenFunc mock
	accountToken := func(ctx context.Context, account ids.AccountID) (entities.ServerConfigReadOnly, *oauth2.Token, error) {
		return serverConfig, accountTokens[account], nil
	}

	h := New(storage, accountToken, WithMaxConnSize(5000))

	testsuite.RunToolClientTests(h,
		testsuite.WithAfterServerSetup(func(u *url.URL) {
			serverConfig = must(entities.NewServerConfig(ids.RandomServerID(), u))
		}),
	)(t)
}

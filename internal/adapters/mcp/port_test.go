package mcp_test

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/toolclient/testsuite"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"

	. "github.com/quenbyako/cynosure/internal/adapters/mcp"
)

func TestToolClientSuite(t *testing.T) {
	var (
		serverConfig  entities.ServerConfigReadOnly
		accountTokens map[ids.AccountID]*oauth2.Token
	)

	// SaveTokenFunc mock
	storage := func(ctx context.Context, account ids.AccountID, token *oauth2.Token) error {
		accountTokens[account] = token
		return nil
	}

	// AccountTokenFunc mock
	accountToken := func(
		ctx context.Context,
		account ids.AccountID,
	) (entities.ServerConfigReadOnly, *oauth2.Token, error) {
		return serverConfig, accountTokens[account], nil
	}

	handler, err := New(t.Context(), storage, accountToken,
		WithMaxConnSize(5000),
		WithUnsafeExternalHTTPClient(http.DefaultTransport),
		WithInternalHTTPClient(http.DefaultTransport),
	)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	testsuite.RunToolClientTests(handler,
		testsuite.WithAfterServerSetup(func(mcpURL *url.URL) {
			serverConfig = must(entities.NewServerConfig(ids.RandomServerID(), mcpURL))
		}),
	)(t)
}

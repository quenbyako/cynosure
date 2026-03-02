package cynosure

import (
	"context"
	"fmt"

	"github.com/goforj/wire"
	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/adapters/gemini"
	"github.com/quenbyako/cynosure/internal/adapters/mcp"
	"github.com/quenbyako/cynosure/internal/adapters/oauth"
	"github.com/quenbyako/cynosure/internal/adapters/ory"
	"github.com/quenbyako/cynosure/internal/adapters/sql"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/identitymanager"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/oauthhandler"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/toolclient"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

var (
	sqlAdapter = wire.NewSet(newSQLAdapter,
		wire.Bind(new(ports.AgentStorageFactory), new(*sql.Adapter)),
		wire.Bind(new(ports.AccountStorageFactory), new(*sql.Adapter)),
		wire.Bind(new(ports.ServerStorageFactory), new(*sql.Adapter)),
		wire.Bind(new(ports.ThreadStorageFactory), new(*sql.Adapter)),
		wire.Bind(new(ports.ToolStorageFactory), new(*sql.Adapter)),
	)
	geminiAdapter = wire.NewSet(newGeminiModel,
		wire.Bind(new(ports.ChatModelFactory), new(*gemini.GeminiModel)),
		wire.Bind(new(ports.ToolSemanticIndexFactory), new(*gemini.GeminiModel)),
	)
	oauthAdapter = wire.NewSet(newOAuthHandler,
		wire.Bind(new(oauthhandler.Factory), new(*oauth.Handler)),
	)
	mcpAdapter = wire.NewSet(newMCPHandler,
		wire.Bind(new(toolclient.PortFactory), new(*mcp.Handler)),
	)
	oryAdapter = wire.NewSet(newOryClient,
		wire.Bind(new(identitymanager.PortFactory), new(*ory.Client)),
	)
)

func newSQLAdapter(ctx context.Context, p *appParams) (*sql.Adapter, error) {
	return sql.New(ctx, p.databaseURL, sql.WithTrace(p.observability))
}

func newMCPHandler(
	p *appParams,
	oauthHandler oauthhandler.PortWrapped,
	servers ports.ServerStorage,
	accounts ports.AccountStorage,
) *mcp.Handler {
	// Create save token callback
	saveToken := func(ctx context.Context, accountID ids.AccountID, token *oauth2.Token) error {
		account, err := accounts.GetAccount(ctx, accountID)
		if err != nil {
			return fmt.Errorf("getting account: %w", err)
		}

		if err := account.UpdateToken(token); err != nil {
			return fmt.Errorf("updating token: %w", err)
		}

		if err := accounts.SaveAccount(ctx, account); err != nil {
			return fmt.Errorf("saving account: %w", err)
		}

		return nil
	}

	// Create account token callback
	accountToken := func(ctx context.Context, accountID ids.AccountID) (entities.ServerConfigReadOnly, *oauth2.Token, error) {
		account, err := accounts.GetAccount(ctx, accountID)
		if err != nil {
			return nil, nil, fmt.Errorf("getting account: %w", err)
		}

		server, err := servers.GetServerInfo(ctx, accountID.Server())
		if err != nil {
			return nil, nil, fmt.Errorf("getting server info: %w", err)
		}

		return server, account.Token(), nil
	}

	return mcp.New(saveToken, accountToken,
		mcp.WithObservability(p.observability),
	)
}

func newGeminiModel(ctx context.Context, p *appParams, log gemini.LogCallbacks) (*gemini.GeminiModel, error) {
	geminiKey, err := p.geminiKey.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting gemini key from secret getter: %w", err)
	}

	return gemini.New(
		ctx,
		&gemini.ClientConfig{
			APIKey: string(geminiKey),
		},
		gemini.WithLogCallbacks(log),
		gemini.WithTrace(p.observability),
	)
}

func newOAuthHandler(p *appParams) *oauth.Handler {
	return oauth.New(
		p.oauthScopes,
		oauth.WithObservability(p.observability),
	)
}

func newOryClient(ctx context.Context, p *appParams) (*ory.Client, error) {
	adminKey, err := p.oryAdminKey.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting ory admin key: %w", err)
	}

	return ory.New(p.oryEndpoint, string(adminKey), ory.WithObservability(p.observability)), nil
}

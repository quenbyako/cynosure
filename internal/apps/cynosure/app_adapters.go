package cynosure

import (
	"context"
	"fmt"

	"github.com/goforj/wire"
	"golang.org/x/oauth2"

	// "github.com/quenbyako/cynosure/internal/adapters/file"
	"github.com/quenbyako/cynosure/internal/adapters/gemini"
	"github.com/quenbyako/cynosure/internal/adapters/mcp"
	"github.com/quenbyako/cynosure/internal/adapters/oauth"
	"github.com/quenbyako/cynosure/internal/adapters/sql"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

var (
	// fileAdapter = wire.NewSet(
	// 	newFileAdapter,
	// 	wire.Bind(new(ports.ModelSettingsStorageFactory), new(*file.File)),
	// 	wire.Bind(new(ports.AccountStorageFactory), new(*file.File)),
	// 	wire.Bind(new(ports.ServerStorageFactory), new(*file.File)),
	// )
	sqlAdapter = wire.NewSet(
		newSQLAdapter,
		wire.Bind(new(ports.AgentStorageFactory), new(*sql.Adapter)),
		wire.Bind(new(ports.AccountStorageFactory), new(*sql.Adapter)),
		wire.Bind(new(ports.ServerStorageFactory), new(*sql.Adapter)),
		wire.Bind(new(ports.ThreadStorageFactory), new(*sql.Adapter)),
		wire.Bind(new(ports.ToolStorageFactory), new(*sql.Adapter)),
		wire.Bind(new(ports.UserStorageFactory), new(*sql.Adapter)),
	)
	geminiAdapter = wire.NewSet(newGeminiModel,
		wire.Bind(new(ports.ChatModelFactory), new(*gemini.GeminiModel)),
		wire.Bind(new(ports.ToolSemanticIndexFactory), new(*gemini.GeminiModel)),
	)
	oauthAdapter = wire.NewSet(newOAuthHandler,
		wire.Bind(new(ports.OAuthHandlerFactory), new(*oauth.Handler)),
	)
	mcpAdapter = wire.NewSet(newMCPHandler,
		wire.Bind(new(ports.ToolClientFactory), new(*mcp.Handler)),
	)
)

// func newFileAdapter(p *appParams) *file.File {
// 	return file.New(p.storagePath)
// }

func newSQLAdapter(ctx context.Context, p *appParams) (*sql.Adapter, error) {
	return sql.NewAdapter(ctx, p.databaseURL)
}

func newMCPHandler(
	p *appParams,
	oauth ports.OAuthHandler,
	servers ports.ServerStorage,
	accounts ports.AccountStorage,
) *mcp.Handler {
	refresher := func(ctx context.Context, server entities.ServerConfigReadOnly, token *oauth2.Token) (*oauth2.Token, error) {
		cfg := server.AuthConfig()
		if cfg == nil {
			return nil, fmt.Errorf("server %v has no OAuth config", server.ID())
		}

		newToken, err := cfg.TokenSource(ctx, token).Token()
		if err != nil {
			return nil, fmt.Errorf("refreshing token via oauth2: %w", err)
		}

		return newToken, nil
	}

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

	return mcp.NewHandler(refresher, saveToken, accountToken)
}

func newGeminiModel(ctx context.Context, p *appParams, log gemini.LogCallbacks) (*gemini.GeminiModel, error) {
	geminiKey, err := p.geminiKey.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting gemini key from secret getter: %w", err)
	}

	return gemini.NewGeminiModel(
		ctx,
		&gemini.ClientConfig{
			APIKey: string(geminiKey),
		},
		gemini.WithLogCallbacks(log),
	)
}

func newOAuthHandler(p *appParams) *oauth.Handler {
	return oauth.New(
		p.oauthScopes,
		oauth.WithTracerProvider(p.observability),
	)
}

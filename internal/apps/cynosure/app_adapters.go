package cynosure

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/time/rate"
	"google.golang.org/genai"

	"github.com/quenbyako/cynosure/internal/adapters/gemini"
	"github.com/quenbyako/cynosure/internal/adapters/inmemory"
	"github.com/quenbyako/cynosure/internal/adapters/mcp"
	"github.com/quenbyako/cynosure/internal/adapters/oauth"
	"github.com/quenbyako/cynosure/internal/adapters/ory"
	"github.com/quenbyako/cynosure/internal/adapters/sql"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

func newSQLAdapter(ctx context.Context, p *appParams) (*sql.Adapter, error) {
	adapter, err := sql.New(ctx, p.storage.databaseURL, sql.WithTrace(p.observability))
	if err != nil {
		return nil, fmt.Errorf("initializing sql adapter: %w", err)
	}

	return adapter, nil
}

func tokenFuncFromAccountStorage(
	accounts ports.AccountStorage,
	servers ports.ServerStorage,
) mcp.AccountTokenFunc {
	return func(
		ctx context.Context,
		accountID ids.AccountID,
	) (
		entities.ServerConfigReadOnly,
		*oauth2.Token,
		error,
	) {
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
}

func newMCPHandler(
	params *appParams,
	servers ports.ServerStorage,
	accounts ports.AccountStorage,
) (*mcp.Handler, error) {
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

	handler, err := mcp.New(saveToken, tokenFuncFromAccountStorage(accounts, servers),
		mcp.WithObservability(params.observability),
	)
	if err != nil {
		return nil, fmt.Errorf("initializing mcp handler: %w", err)
	}

	return handler, nil
}

func newGeminiModel(
	ctx context.Context, params *appParams, log gemini.LogCallbacks,
) (
	*gemini.GeminiModel, error,
) {
	geminiKey, err := params.gemini.key.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting gemini key from secret getter: %w", err)
	}

	var emptyHTTPOptions genai.HTTPOptions

	model, err := gemini.New(
		ctx,
		&genai.ClientConfig{
			APIKey:      string(geminiKey),
			Backend:     0,
			Project:     "",
			Location:    "",
			Credentials: nil,
			HTTPClient:  nil,
			HTTPOptions: emptyHTTPOptions,
		},
		gemini.WithLogCallbacks(log),
		gemini.WithTrace(params.observability),
	)
	if err != nil {
		return nil, fmt.Errorf("initializing gemini model: %w", err)
	}

	return model, nil
}

func newOAuthHandler(p *appParams) *oauth.Handler {
	return oauth.New(
		p.ory.oauthScopes,
		oauth.WithObservability(p.observability),
	)
}

func newOryClient(ctx context.Context, params *appParams) (*ory.Client, error) {
	adminKey, err := params.ory.adminKey.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting ory admin key: %w", err)
	}

	clientSecret, err := params.ory.clientSecret.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting ory client secret: %w", err)
	}

	return ory.New(params.ory.endpoint, string(adminKey),
		ory.WithObservability(params.observability),
		ory.WithClientCredentials(params.ory.clientID, string(clientSecret)),
		ory.WithScopes(params.ory.scopes...),
		ory.WithRedirectURL(params.ory.redirectURL),
	), nil
}

const (
	defaultQuotaDrop  = time.Hour
	defaultQuotaBurst = 20
)

func newRateLimiter(ctx context.Context, params *appParams) *inmemory.RateLimiter {
	return inmemory.NewRateLimiter(
		rate.Every(defaultQuotaDrop),
		defaultQuotaBurst,
		time.Now,
		params.observability,
	)
}

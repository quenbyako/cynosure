package cynosure

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"google.golang.org/genai"

	"github.com/quenbyako/cynosure/internal/adapters/gemini"
	"github.com/quenbyako/cynosure/internal/adapters/inmemory"
	"github.com/quenbyako/cynosure/internal/adapters/mcp"
	"github.com/quenbyako/cynosure/internal/adapters/oauth"
	"github.com/quenbyako/cynosure/internal/adapters/ory"
	"github.com/quenbyako/cynosure/internal/adapters/redis"
	"github.com/quenbyako/cynosure/internal/adapters/sql"
	"github.com/quenbyako/cynosure/internal/apps/cynosure/refreshtoken"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/oauthhandler"
)

func newPostgres(params *appParams, lifecycle *lifecycle) constructor[*sql.Adapter] {
	return construct(func(ctx context.Context) (*sql.Adapter, error) {
		adapter, err := sql.New(ctx, params.storage.databaseURL, sql.WithObservability(params.observability))
		if err != nil {
			return nil, fmt.Errorf("initializing sql adapter: %w", err)
		}

		lifecycle.schedule(func(ctx context.Context) error {
			<-ctx.Done()

			return adapter.Close()
		})

		return adapter, nil
	})
}

func newOauthRefresher(
	ctx context.Context,
	lifecycle *lifecycle,
	accounts constructor[ports.AccountStorage],
	servers constructor[ports.ServerStorage],
	oauthPort oauthhandler.PortWrapped,
) (*refreshtoken.RefreshConstructor, error) {
	accountsWrapper, err := accounts.Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("building accounts wrapper: %w", err)
	}

	serversWrapper, err := servers.Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("building servers wrapper: %w", err)
	}

	constructor := refreshtoken.NewConstructor(
		oauthPort,
		accountsWrapper,
		serversWrapper,
		defaultWorkersCount,
	)

	lifecycle.schedule(constructor.Run)

	return constructor, nil
}

func newMCPHandler(
	ctx context.Context,
	params *appParams,
	lifecycle *lifecycle,
	refresher *refreshtoken.RefreshConstructor,
) (*mcp.Handler, error) {
	handler, err := mcp.New(ctx, refresher.Token, refresher.Build,
		mcp.WithObservability(params.observability),
		mcp.WithInternalHTTPClient(params.internalMcpClient),
		mcp.WithExternalHTTPClient(params.externalMcpClient),
	)
	if err != nil {
		return nil, fmt.Errorf("initializing mcp handler: %w", err)
	}

	lifecycle.schedule(func(ctx context.Context) error {
		<-ctx.Done()

		return handler.Close()
	})

	return handler, nil
}

func newGeminiModel(
	ctx context.Context, params *appParams, log gemini.LogCallbacks,
) (
	*gemini.GeminiModel, error,
) {
	model, err := gemini.New(ctx,
		newGeminiConfig(params.gemini.key, params.gemini.apiClient),
		gemini.WithLogCallbacks(log),
		gemini.WithTrace(params.observability),
		gemini.WithMaxMessagesPerRequest(params.chat.historyLimit),
		gemini.WithChatInputLimit(params.rate.chatInputGlobal),
		gemini.WithEmbeddingLimit(params.rate.embeddingGlobal),
	)
	if err != nil {
		return nil, fmt.Errorf("initializing gemini model: %w", err)
	}

	return model, nil
}

func newGeminiConfig(key SecretGetter, client http.RoundTripper) *genai.ClientConfig {
	return &genai.ClientConfig{
		APIKey:      "ROTATED", // genai requires non-empty key, but we override it in transport
		Backend:     0,
		Project:     "",
		Location:    "",
		Credentials: nil,
		HTTPClient: &http.Client{
			Transport: &rotatedKeyTransport{
				base:   client,
				getter: key,
			},
			CheckRedirect: nil,
			Jar:           nil,
			Timeout:       0,
		},
		HTTPOptions: genai.HTTPOptions{
			BaseURL:               "",
			BaseURLResourceScope:  "",
			APIVersion:            "",
			Headers:               nil,
			Timeout:               nil,
			ExtraBody:             nil,
			ExtrasRequestProvider: nil,
		},
	}
}

type rotatedKeyTransport struct {
	base   http.RoundTripper
	getter SecretGetter
}

func (t *rotatedKeyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	key, err := t.getter.Get(req.Context())
	if err != nil {
		return nil, fmt.Errorf("getting api key: %w", err)
	}

	req.Header.Set("X-Goog-Api-Key", string(key))

	//nolint:wrapcheck // implementing RoundTripper
	return t.base.RoundTrip(req)
}

func newOAuthHandler(params *appParams) *oauth.Handler {
	return oauth.New(
		params.ory.oauthScopes,
		oauth.WithObservability(params.observability),
		// Note: using mcp clients, as it's using only for mcp clients.
		// Authorization is related only to MCP and nothing more.
		oauth.WithTransports(params.internalMcpClient, params.externalMcpClient),
	)
}

func newOryClient(ctx context.Context, params *appParams) (*ory.Adapter, error) {
	adminKey, err := params.ory.adminKey.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting ory admin key: %w", err)
	}

	clientSecret, err := params.ory.clientSecret.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting ory client secret: %w", err)
	}

	client, err := ory.New(params.ory.endpoint, string(adminKey),
		ory.WithObservability(params.observability),
		ory.WithClientCredentials(params.ory.clientID, string(clientSecret)),
		ory.WithScopes(params.ory.scopes...),
		ory.WithRedirectURL(params.ory.redirectURL),
		ory.WithHTTPClient(params.ory.apiClient),
	)
	if err != nil {
		return nil, fmt.Errorf("initializing ory client: %w", err)
	}

	return client, nil
}

func newInmem(params *appParams, lifecycle *lifecycle) constructor[*inmemory.RateLimiter] {
	return construct(func(_ context.Context) (*inmemory.RateLimiter, error) {
		adapter := inmemory.NewRateLimiter(
			params.rate.chatInput.Period(), params.rate.chatInput.Limit(),
			params.rate.chatOutput.Period(), params.rate.chatOutput.Limit(),
			params.rate.embedding.Period(), params.rate.embedding.Limit(),
			params.rate.maxWait,
			time.Now,
			params.observability,
		)

		lifecycle.schedule(adapter.StartCleanupJob)

		return adapter, nil
	})
}

func newRedis(params *appParams) constructor[*redis.RateLimiter] {
	if params.redis.client == nil {
		return &noopConstructor[*redis.RateLimiter]{}
	}

	return construct(func(_ context.Context) (*redis.RateLimiter, error) {
		return redis.New(
			params.redis.client,
			params.rate.chatInput.Period(), params.rate.chatInput.Limit(),
			params.rate.chatOutput.Period(), params.rate.chatOutput.Limit(),
			params.rate.embedding.Period(), params.rate.embedding.Limit(),
			params.rate.maxWait,
			params.observability,
		), nil
	})
}

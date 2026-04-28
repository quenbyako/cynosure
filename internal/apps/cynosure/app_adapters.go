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
	"github.com/quenbyako/cynosure/internal/adapters/sql"
	"github.com/quenbyako/cynosure/internal/apps/cynosure/refreshtoken"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/oauthhandler"
)

func newSQLAdapter(ctx context.Context, p *appParams) (*sql.Adapter, error) {
	adapter, err := sql.New(ctx, p.storage.databaseURL, sql.WithTrace(p.observability))
	if err != nil {
		return nil, fmt.Errorf("initializing sql adapter: %w", err)
	}

	return adapter, nil
}

func newOauthRefresher(
	accounts ports.AccountStorage,
	servers ports.ServerStorage,
	oauth oauthhandler.PortWrapped,
) *refreshtoken.RefreshConstructor {
	return refreshtoken.NewConstructor(
		oauth,
		accounts,
		servers,
		4, // todo: make it dynamic?
	)
}

func newMCPHandler(
	ctx context.Context,
	params *appParams,
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
		gemini.WithHardCap(params.chat.hardCap),
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

func newOAuthHandler(p *appParams) *oauth.Handler {
	return oauth.New(
		p.ory.oauthScopes,
		oauth.WithObservability(p.observability),
		// Note: using mcp clients, as it's using only for mcp clients.
		// Authorization is related only to MCP and nothing more.
		oauth.WithTransports(p.internalMcpClient, p.externalMcpClient),
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

const (
	ttlPeriodMultiplier = 2
)

func newRateLimiter(params *appParams) *inmemory.RateLimiter {
	limit := params.rateLimit.Limit()
	burst := params.rateLimit.Burst()

	return inmemory.NewRateLimiter(
		limit,
		burst,
		params.rateLimit.Period()*ttlPeriodMultiplier,
		time.Now,
		params.observability,
	)
}

package openrouter

import (
	"context"
	"fmt"
	"net/http"

	openroutersdk "github.com/OpenRouterTeam/go-sdk"
	"github.com/OpenRouterTeam/go-sdk/models/components"
	"github.com/quenbyako/core"
	"github.com/quenbyako/cynosure/contrib/core-params/ratelimit"
	"github.com/quenbyako/cynosure/contrib/tokencounter"
	"github.com/quenbyako/ext/fs"
	"golang.org/x/time/rate"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/chatmodel"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/embedding"
)

// Adapter implements OpenRouter adapter.
type Adapter struct {
	obs              observable
	apiKey           SecretGetter
	sdkClient        *openroutersdk.OpenRouter
	httpClient       *http.Client
	embeddingLimiter *rate.Limiter
	chatInputLimiter *rate.Limiter
	tokenCounter     *tokencounter.TokenCounter
	maxMsgsPerReq    uint
}

var (
	_ chatmodel.PortFactory = (*Adapter)(nil)
	_ embedding.PortFactory = (*Adapter)(nil)
)

func (o *Adapter) Embedding() embedding.PortWrapped { return embedding.Wrap(o, o.obs.stack) }
func (o *Adapter) ChatModel() chatmodel.PortWrapped { return chatmodel.Wrap(o, o.obs.stack) }

type newParams struct {
	traceProvider  core.Metrics
	transport      http.RoundTripper
	fsys           fs.WFS
	embeddingLimit ratelimit.Policy
	chatInputLimit ratelimit.Policy
	maxMsgsPerReq  uint
}

// NewOption defines functional option for New.
type NewOption func(*newParams)

func buildNewParams(opts ...NewOption) newParams {
	params := newParams{
		traceProvider:  core.NoopMetrics(),
		maxMsgsPerReq:  defaultHardCap,
		embeddingLimit: ratelimit.Policy{},
		chatInputLimit: ratelimit.Policy{},
		transport:      http.DefaultTransport,
		fsys:           fs.DirFS("/tmp/tokenizers"),
	}

	for _, opt := range opts {
		opt(&params)
	}

	return params
}

// WithTrace sets trace.TracerProvider for OpenRouter model.
func WithTrace(traceProvider core.Metrics) NewOption {
	return func(params *newParams) { params.traceProvider = traceProvider }
}

// WithMaxMessagesPerRequest sets systemic messages limit for OpenRouter model.
func WithMaxMessagesPerRequest(limit uint) NewOption {
	return func(params *newParams) { params.maxMsgsPerReq = limit }
}

// WithEmbeddingLimit sets global rate limit for embeddings.
func WithEmbeddingLimit(limit ratelimit.Policy) NewOption {
	return func(params *newParams) { params.embeddingLimit = limit }
}

// WithChatInputLimit sets global rate limit for chat input tokens.
func WithChatInputLimit(limit ratelimit.Policy) NewOption {
	return func(params *newParams) { params.chatInputLimit = limit }
}

// WithTransport sets custom round tripper.
func WithTransport(transport http.RoundTripper) NewOption {
	return func(params *newParams) { params.transport = transport }
}

// WithCacheDir sets the cache directory for Hugging Face tokenizers.
func WithCacheDir(dir string) NewOption {
	return func(params *newParams) { params.fsys = fs.DirFS(dir) }
}

// WithFS sets custom filesystem for Hugging Face tokenizers cache.
func WithFS(fsys fs.WFS) NewOption {
	return func(params *newParams) { params.fsys = fsys }
}

// New creates a new OpenRouter adapter.
func New(ctx context.Context, apiKey SecretGetter, opts ...NewOption) (*Adapter, error) {
	params := buildNewParams(opts...)
	obs := newObservable(ports.StackFromCore(params.traceProvider, pkgName))

	httpClient := newHTTPClient(params.transport, apiKey)

	tc, err := tokencounter.NewTokenCounter(params.fsys, httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize token counter: %w", err)
	}

	sdkClient := newSDKClient(httpClient, apiKey)

	model := &Adapter{
		obs:              obs,
		apiKey:           apiKey,
		sdkClient:        sdkClient,
		httpClient:       httpClient,
		maxMsgsPerReq:    params.maxMsgsPerReq,
		embeddingLimiter: newRateLimiter(params.embeddingLimit),
		chatInputLimiter: newRateLimiter(params.chatInputLimit),
		tokenCounter:     tc,
	}

	if err = model.initAndPing(ctx); err != nil {
		return nil, err
	}

	return model, nil
}

func newHTTPClient(transport http.RoundTripper, apiKey SecretGetter) *http.Client {
	retriable := []int{
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
	}

	return &http.Client{
		Transport:     newRetryTransport(transport, apiKey, retriable),
		Timeout:       httpClientTimeout,
		CheckRedirect: nil,
		Jar:           nil,
	}
}

func newSDKClient(httpClient *http.Client, apiKey SecretGetter) *openroutersdk.OpenRouter {
	return openroutersdk.New(
		openroutersdk.WithSecuritySource(func(ctx context.Context) (components.Security, error) {
			key, err := apiKey.Get(ctx)
			if err != nil {
				return components.Security{}, fmt.Errorf("getting api key for sdk: %w", err)
			}

			keyStr := string(key)

			return components.Security{APIKey: &keyStr}, nil
		}),
		openroutersdk.WithClient(httpClient),
	)
}

func (o *Adapter) initAndPing(ctx context.Context) error {
	if err := o.validate(); err != nil {
		return err
	}

	if err := o.ping(ctx); err != nil {
		return fmt.Errorf("can't connect to OpenRouter API: %w", err)
	}

	return nil
}

type SecretGetter interface {
	Get(ctx context.Context) ([]byte, error)
}

func newRateLimiter(p ratelimit.Policy) *rate.Limiter {
	return rate.NewLimiter(p.Rate(), p.Limit())
}

// ping verifies connectivity and API key validity by fetching model list.
func (o *Adapter) ping(ctx context.Context) error {
	if _, err := o.sdkClient.Models.List(ctx, nil, nil, nil); err != nil {
		return fmt.Errorf("openrouter list models: %w", err)
	}

	return nil
}

func (o *Adapter) validate() error {
	if o.tokenCounter == nil {
		return fmt.Errorf("%w: tokenCounter is nil", ErrInternalValidation)
	}

	return nil
}

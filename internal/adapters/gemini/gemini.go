package gemini

import (
	"context"
	"fmt"
	"time"

	"github.com/quenbyako/core"
	"github.com/quenbyako/cynosure/contrib/core-params/ratelimit"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/time/rate"
	"google.golang.org/genai"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/chatmodel"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/embedding"
)

const (
	thinkingBudget = 32
	defaultHardCap = 50

	maxLimiterWait = 2 * time.Second
)

// GeminiModel implements Gemini adapter.
type GeminiModel struct {
	log              LogCallbacks
	trace            trace.Tracer
	tracer           ports.ObserveStack
	client           *genai.Client
	thinkingConfig   *genai.ThinkingConfig
	embeddingLimiter *rate.Limiter
	chatInputLimiter *rate.Limiter
	maxMsgsPerReq    uint
}

var (
	_ chatmodel.PortFactory = (*GeminiModel)(nil)
	_ embedding.PortFactory = (*GeminiModel)(nil)
)

func (g *GeminiModel) Embedding() embedding.PortWrapped { return embedding.Wrap(g, g.tracer) }
func (g *GeminiModel) ChatModel() chatmodel.PortWrapped { return chatmodel.Wrap(g, g.tracer) }

type newParams struct {
	log            LogCallbacks
	traceProvider  core.Metrics
	maxMsgsPerReq  uint
	embeddingLimit ratelimit.Policy
	chatInputLimit ratelimit.Policy
}

// NewOption defines functional option for New.
type NewOption func(*newParams)

func buildNewParams(opts ...NewOption) newParams {
	params := newParams{
		log:            NoOpLogCallbacks{},
		traceProvider:  core.NoopMetrics(),
		maxMsgsPerReq:  defaultHardCap, // default fallback
		embeddingLimit: ratelimit.Policy{},
		chatInputLimit: ratelimit.Policy{},
	}

	for _, opt := range opts {
		opt(&params)
	}

	return params
}

// WithLogCallbacks sets LogCallbacks for Gemini model.
func WithLogCallbacks(log LogCallbacks) NewOption {
	return func(params *newParams) { params.log = log }
}

// WithTrace sets trace.TracerProvider for Gemini model.
func WithTrace(traceProvider core.Metrics) NewOption {
	return func(params *newParams) { params.traceProvider = traceProvider }
}

// WithMaxMessagesPerRequest sets systemic messages limit for Gemini model.
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

// New creates a new Gemini adapter.
func New(ctx context.Context, cfg *genai.ClientConfig, opts ...NewOption) (*GeminiModel, error) {
	params := buildNewParams(opts...)

	client, err := genai.NewClient(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create GenAI client: %w", err)
	}

	model := &GeminiModel{
		client:           client,
		thinkingConfig:   defaultThinkingConfig(),
		log:              params.log,
		trace:            params.traceProvider.Tracer(pkgName),
		tracer:           ports.StackFromCore(params.traceProvider, pkgName),
		maxMsgsPerReq:    params.maxMsgsPerReq,
		embeddingLimiter: newRateLimiter(params.embeddingLimit),
		chatInputLimiter: newRateLimiter(params.chatInputLimit),
	}

	if err := model.validate(); err != nil {
		return nil, err
	}

	if err := model.ping(ctx); err != nil {
		return nil, fmt.Errorf("can't connect to Google API: %w", err)
	}

	return model, nil
}

func defaultThinkingConfig() *genai.ThinkingConfig {
	return &genai.ThinkingConfig{
		IncludeThoughts: true,
		ThinkingBudget:  ptr(int32(thinkingBudget)),
		ThinkingLevel:   "",
	}
}

func newRateLimiter(p ratelimit.Policy) *rate.Limiter {
	return rate.NewLimiter(p.Rate(), p.Limit())
}

// ping verifies connectivity and API key validity.
func (g *GeminiModel) ping(ctx context.Context) error {
	_, err := g.client.Models.List(ctx, nil)
	if err != nil {
		return fmt.Errorf("gemini ping failed: %w", err)
	}

	return nil
}

func (g *GeminiModel) validate() error {
	if g.client == nil {
		return ErrInternalValidation("client is nil")
	}

	if g.thinkingConfig == nil {
		return ErrInternalValidation("thinkingConfig is nil")
	}

	if g.log == nil {
		return ErrInternalValidation("log is nil")
	}

	if g.trace == nil {
		return ErrInternalValidation("trace is nil")
	}

	return nil
}

func ptr[T any](v T) *T { return &v }

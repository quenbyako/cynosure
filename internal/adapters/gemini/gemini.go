package gemini

import (
	"context"
	"fmt"

	"github.com/quenbyako/core"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/genai"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/chatmodel"
)

const (
	thinkingBudget = 32
)

// GeminiModel implements Gemini adapter.
type GeminiModel struct {
	client         *genai.Client
	thinkingConfig *genai.ThinkingConfig

	log    LogCallbacks
	trace  trace.Tracer
	tracer ports.ObserveStack
}

var (
	_ chatmodel.PortFactory          = (*GeminiModel)(nil)
	_ ports.ToolSemanticIndexFactory = (*GeminiModel)(nil)
)

// ToolSemanticIndex returns ports.ToolSemanticIndex interface.
//
//nolint:ireturn // it's a port interface
func (g *GeminiModel) ToolSemanticIndex() ports.ToolSemanticIndex { return g }

// ChatModel returns ports.ChatModel interface.
//
//nolint:ireturn // it's a port interface
func (g *GeminiModel) ChatModel() chatmodel.PortWrapped { return chatmodel.Wrap(g, g.tracer) }

type newParams struct {
	log           LogCallbacks
	traceProvider core.Metrics
}

// NewOption defines functional option for New.
type NewOption func(*newParams)

func buildNewParams(opts ...NewOption) newParams {
	params := newParams{
		log:           NoOpLogCallbacks{},
		traceProvider: core.NoopMetrics(),
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

// New creates a new Gemini adapter.
func New(ctx context.Context, cfg *genai.ClientConfig, opts ...NewOption) (*GeminiModel, error) {
	params := buildNewParams(opts...)

	client, err := genai.NewClient(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create GenAI client: %w", err)
	}

	model := GeminiModel{
		client: client,
		thinkingConfig: &genai.ThinkingConfig{
			IncludeThoughts: true,
			ThinkingBudget:  ptr(int32(thinkingBudget)),
			ThinkingLevel:   "",
		},
		log:    params.log,
		trace:  params.traceProvider.Tracer(pkgName),
		tracer: ports.StackFromCore(params.traceProvider, pkgName),
	}

	if err := model.validate(); err != nil {
		return nil, err
	}

	return &model, nil
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

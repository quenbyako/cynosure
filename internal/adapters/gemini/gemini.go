package gemini

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel/trace"
	noopTrace "go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/genai"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
)

const pkgName = "github.com/quenbyako/cynosure/internal/adapters/gemini"

type ClientConfig = genai.ClientConfig

type GeminiModel struct {
	client         *genai.Client
	thinkingConfig *genai.ThinkingConfig

	log   LogCallbacks
	trace trace.Tracer
}

var (
	_ ports.ChatModelFactory         = (*GeminiModel)(nil)
	_ ports.ToolSemanticIndexFactory = (*GeminiModel)(nil)
)

func (g *GeminiModel) ToolSemanticIndex() ports.ToolSemanticIndex { return g }
func (g *GeminiModel) ChatModel() ports.ChatModel                 { return g }

type newParams struct {
	log   LogCallbacks
	trace trace.TracerProvider
}

type NewOption func(*newParams)

func WithLogCallbacks(log LogCallbacks) NewOption {
	return func(g *newParams) { g.log = log }
}

func WithTrace(tp trace.TracerProvider) NewOption {
	return func(g *newParams) {
		if tp == nil {
			panic("tracer provider is nil")
		}

		g.trace = tp
	}
}

func New(ctx context.Context, cfg *ClientConfig, opts ...NewOption) (*GeminiModel, error) {
	p := newParams{
		log:   NoOpLogCallbacks{},
		trace: noopTrace.NewTracerProvider(),
	}
	for _, opt := range opts {
		opt(&p)
	}

	client, err := genai.NewClient(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create GenAI client: %w", err)
	}

	m := GeminiModel{
		client: client,
		thinkingConfig: &genai.ThinkingConfig{
			IncludeThoughts: true,
			ThinkingBudget:  ptr[int32](32),
			ThinkingLevel:   "",
		},

		log:   p.log,
		trace: p.trace.Tracer(pkgName),
	}
	if err := m.validate(); err != nil {
		return nil, err
	}

	return &m, nil
}

func (g *GeminiModel) validate() error {
	if g.client == nil {
		return errors.New("client is nil")
	}

	if g.thinkingConfig == nil {
		return errors.New("thinkingConfig is nil")
	}

	if g.log == nil {
		return errors.New("log is nil")
	}

	if g.trace == nil {
		return errors.New("trace is nil")
	}

	return nil
}

func ptr[T any](v T) *T { return &v }

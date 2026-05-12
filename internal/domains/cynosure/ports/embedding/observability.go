package embedding

import (
	"context"

	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/trace"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
)

type observable struct {
	t trace.Tracer
	l log.Logger
}

func newObservable(stack ports.ObserveStack) *observable {
	if stack == nil {
		stack = ports.NoOpObserveStack()
	}

	return &observable{
		t: stack.Tracer(),
		l: stack.Logger(),
	}
}

// trace callbacks

func (o *observable) indexTool(
	ctx context.Context,
) (context.Context, span) {
	//nolint:spancheck // safe to omit, since wrapper automatically closes span.
	ctx, span := o.t.Start(ctx, "cynosure.ports.embedding.index_tool",
		trace.WithSpanKind(trace.SpanKindInternal),
	)

	//nolint:spancheck // see above
	return ctx, &spanCallback{span: span}
}

func (o *observable) buildToolEmbedding(
	ctx context.Context,
) (context.Context, span) {
	//nolint:spancheck // safe to omit, since wrapper automatically closes span.
	ctx, span := o.t.Start(ctx, "cynosure.ports.embedding.build_tool_embedding",
		trace.WithSpanKind(trace.SpanKindInternal),
	)

	//nolint:spancheck // see above
	return ctx, &spanCallback{span: span}
}

// log callbacks

// generic span

type span interface {
	end()
	recordError(err error)
}

type spanCallback struct {
	span trace.Span
}

func (c *spanCallback) end() {
	if c != nil && c.span != nil {
		c.span.End()
	}
}

func (c *spanCallback) recordError(err error) {
	if err != nil && c != nil && c.span != nil {
		c.span.RecordError(err)
	}
}

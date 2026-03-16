package toolclient

import (
	"context"
	"encoding/json"
	"log/slog"

	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.39.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
)

//nolint:gosec // lmao, it's not a credential
const (
	attrAccountID attribute.Key = "cynosure.account.id"
	connHasToken  attribute.Key = "cynosure.mcp.has_token"
)

type observable struct {
	t trace.Tracer
	l slog.Handler
}

func newObservable(stack ports.ObserveStack) *observable {
	if stack == nil {
		panic("required observable stack")
	}

	return &observable{
		t: stack.Tracer(),
		l: stack.Logger(),
	}
}

// trace callbacks

type discoverToolsCallback interface {
	span
}

func (o *observable) discoverTools(
	ctx context.Context,
	accountID, serverURL string,
	hasToken bool,
) (context.Context, discoverToolsCallback) {
	ctx, span := o.t.Start(ctx, "cynosure.ports.tool.discover_tools",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			semconv.McpResourceURI(serverURL),
			attrAccountID.String(accountID),
			connHasToken.Bool(hasToken),
		),
	)

	return ctx, &spanCallback{span: span}
}

type executeToolCallback interface {
	recordResponse(json.RawMessage)
	span
}

type executeToolSpan struct {
	spanCallback
}

func (c *executeToolSpan) recordResponse(response json.RawMessage) {
	if c != nil {
		c.span.SetAttributes(semconv.GenAIToolCallResultKey.String(string(response)))
	}
}

func (o *observable) executeTool(
	ctx context.Context,
	toolName string,
	args map[string]json.RawMessage,
	toolCallID string,
) (context.Context, executeToolCallback) {
	serialized, _ := json.Marshal(args)

	ctx, span := o.t.Start(ctx, "cynosure.ports.tool.execute_tool",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			semconv.GenAIToolCallArgumentsKey.String(string(serialized)),
			semconv.GenAIToolCallID(toolCallID),
			semconv.GenAIToolName(toolName),
			semconv.GenAIToolType("function"),
		),
	)

	return ctx, &executeToolSpan{spanCallback: spanCallback{span: span}}
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
	if c.span != nil {
		c.span.End()
	}
}

func (c *spanCallback) recordError(err error) {
	if err != nil && c.span != nil {
		c.span.RecordError(err)
	}
}

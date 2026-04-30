package chat

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/semconv/v1.40.0/genaiconv"
	"go.opentelemetry.io/otel/trace"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

const (
	eventMaxTurnsReached = "generate.max_turns_reached"
	eventToolCalled      = "generate.tool_called"
)

type observable struct {
	now func() time.Time
	t   trace.Tracer
	l   log.Logger

	tokenUsage        genaiconv.ClientTokenUsage
	operationDuration genaiconv.ClientOperationDuration
}

func newObservable(stack ports.ObserveStack) (observable, error) {
	if stack == nil {
		stack = ports.NoOpObserveStack()
	}

	meter := stack.Meter()

	tokenUsage, err := genaiconv.NewClientTokenUsage(meter)
	if err != nil {
		return observable{}, fmt.Errorf("client token usage meter: %w", err)
	}

	operationDuration, err := genaiconv.NewClientOperationDuration(meter)
	if err != nil {
		return observable{}, fmt.Errorf("client operation duration meter: %w", err)
	}

	return observable{
		now: time.Now,
		t:   stack.Tracer(),
		l:   stack.Logger(),

		tokenUsage:        tokenUsage,
		operationDuration: operationDuration,
	}, nil
}

// log callbacks

func (o *observable) maxTurnsReached(ctx context.Context, threadID string) {
	o.event(ctx, log.SeverityWarn, eventMaxTurnsReached).
		Context(
			attribute.Key("thread_id").String(threadID),
		).
		Msg("Model reached max turns with tool calls, consider adjusting settings")
}

func (o *observable) toolCalled(
	ctx context.Context, threadID string, toolNames []messages.MessageToolRequest,
) {
	names := make([]string, len(toolNames))
	for i, req := range toolNames {
		names[i] = req.ToolName()
	}

	o.event(ctx, log.SeverityInfo, eventToolCalled).
		Context(
			attribute.Key("thread_id").String(threadID),
			attribute.Key("tool_names").StringSlice(names),
		).
		Msg("Tool called during generation")
}

// metric callbacks

func (o *observable) recordUsage(
	ctx context.Context, model string, inputTokens, outputTokens uint32, duration time.Duration,
) {
	o.tokenUsage.Record(ctx, int64(inputTokens),
		genaiconv.OperationNameChat,
		genaiconv.ProviderNameGCPGenAI,
		genaiconv.TokenTypeInput,
		semconv.GenAIRequestModel(model),
		// TODO: get response model from... response? is there a way?
		semconv.GenAIResponseModel(model),
	)
	o.tokenUsage.Record(ctx, int64(outputTokens),
		genaiconv.OperationNameChat,
		genaiconv.ProviderNameGCPGenAI,
		genaiconv.TokenTypeOutput,
		semconv.GenAIRequestModel(model),
		// TODO: get response model from... response? is there a way?
		semconv.GenAIResponseModel(model),
	)

	o.operationDuration.Record(ctx, duration.Seconds(),
		genaiconv.OperationNameChat,
		genaiconv.ProviderNameGCPGenAI,
		semconv.GenAIRequestModel(model),
		// TODO: get response model from... response? is there a way?
		semconv.GenAIResponseModel(model),
	)

	// Also add to span if exists
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		semconv.GenAIUsageInputTokens(int(inputTokens)),
		semconv.GenAIUsageOutputTokens(int(outputTokens)),
	)
}

// trace callbacks

//nolint:spancheck,ireturn // intentional polymorphism: returns internal span interface
func (o *observable) generateResponse(ctx context.Context) (context.Context, span) {
	ctx, span := o.t.Start(ctx, "cynosure.usecases.generate_response",
		trace.WithSpanKind(trace.SpanKindInternal),
	)

	return ctx, &spanCallback{span: span}
}

//nolint:spancheck,ireturn // intentional polymorphism: returns internal span interface
func (o *observable) agentLoop(ctx context.Context) (context.Context, span) {
	ctx, span := o.t.Start(ctx, "cynosure.usecases.agent_loop",
		trace.WithSpanKind(trace.SpanKindInternal),
	)

	return ctx, &spanCallback{span: span}
}

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

// generic event

type eventBuilder struct {
	//nolint:containedctx // it's impossible in other ways provide context values
	ctx context.Context
	h   log.Logger
	r   log.Record
}

func (o *observable) event(ctx context.Context, level log.Severity, eventType string) *eventBuilder {
	if !o.l.Enabled(ctx, log.EnabledParameters{Severity: level, EventName: eventType}) {
		return nil
	}

	event := log.Record{}
	event.SetTimestamp(o.now())
	event.SetSeverity(level)
	event.SetEventName(eventType)

	// Note: here we are not providing trace context, service name, version,
	// etc, since sdk logger already provides that features outside of body. By
	// term of otel logs, "context" is an "attributes" object, which means
	// absolutely same logic, as provided in 12 factors.

	return &eventBuilder{ctx: ctx, h: o.l, r: event}
}

func (e *eventBuilder) Context(attrs ...attribute.KeyValue) *eventBuilder {
	logattrs := make([]log.KeyValue, len(attrs))
	for i, attr := range attrs {
		logattrs[i] = log.KeyValueFromAttribute(attr)
	}

	if e != nil {
		e.r.AddAttributes(logattrs...)
	}

	return e
}

func (e *eventBuilder) Msgf(format string, v ...any) { e.Msg(fmt.Sprintf(format, v...)) }
func (e *eventBuilder) Msg(msg string) {
	e.r.SetBody(log.StringValue(msg))
	e.h.Emit(e.ctx, e.r)
}

func omitOK[T any](s T, ok bool) T {
	if !ok {
		return *new(T)
	}

	return s
}

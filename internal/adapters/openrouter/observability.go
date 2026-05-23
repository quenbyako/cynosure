package openrouter

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
)

type observable struct {
	stack ports.ObserveStack
	now   func() time.Time
	l     log.Logger
}

func newObservable(stack ports.ObserveStack) observable {
	if stack == nil {
		stack = ports.NoOpObserveStack()
	}

	return observable{
		stack: stack,
		now:   time.Now,
		l:     stack.Logger(),
	}
}

func (o *observable) streamStarted(ctx context.Context, model string, toolCount int) {
	o.event(ctx, log.SeverityInfo, "cynosure.adapters.openrouter.stream_started").
		Context(
			attribute.String("model", model),
			attribute.Int("tool_count", toolCount),
		).
		Msg("Passing tools to OpenRouter")
}

func (o *observable) tokenCountMismatch(ctx context.Context, model string, expected, got uint32) {
	o.event(ctx, log.SeverityWarn, "cynosure.adapters.openrouter.token_count_mismatch").
		Context(
			attribute.String("model", model),
			attribute.Int("expected", int(expected)),
			attribute.Int("got", int(got)),
		).
		Msg("Token count mismatch")
}

// generic event

type eventBuilder struct {
	//nolint:containedctx // it's impossible in other ways provide context values
	ctx context.Context
	h   log.Logger
	r   log.Record
}

func (o *observable) event(ctx context.Context, level log.Severity, name string) *eventBuilder {
	if !o.l.Enabled(ctx, log.EnabledParameters{Severity: level, EventName: name}) {
		return nil
	}

	event := log.Record{}
	event.SetTimestamp(o.now())
	event.SetSeverity(level)
	event.SetEventName(name)

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
	if e == nil {
		return
	}

	e.r.SetBody(log.StringValue(msg))
	e.h.Emit(e.ctx, e.r)
}

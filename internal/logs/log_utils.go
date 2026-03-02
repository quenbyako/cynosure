package logs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/quenbyako/core"
	"github.com/quenbyako/core/contrib/runtime"
	"github.com/quenbyako/cynosure/contrib/onelog"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.39.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/quenbyako/cynosure/internal/adapters/gemini"
	"github.com/quenbyako/cynosure/internal/controllers/telegram"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/chat"
)

func New(l onelog.Logger) *BaseLogger { return &BaseLogger{l: l} }

type BaseLogger struct {
	l  onelog.Logger
	ll slog.Handler
}

var _ chat.LogCallbacks = (*BaseLogger)(nil)
var _ gemini.LogCallbacks = (*BaseLogger)(nil)
var _ telegram.LogCallbacks = (*BaseLogger)(nil)
var _ runtime.LogCallbacks = (*BaseLogger)(nil)

type eventBuilder struct {
	ctx context.Context
	h   slog.Handler
	r   slog.Record
}

func (l *BaseLogger) event(ctx context.Context, level slog.Level, eventType string) *eventBuilder {
	if !l.ll.Enabled(ctx, level) {
		return nil
	}

	app, _ := core.AppNameFromContext(ctx)
	version, _ := core.VersionFromContext(ctx)

	event := slog.NewRecord(time.Now(), level, "", 0)
	event.AddAttrs(
		slog.String("service", omitOK(app.Name())),
		slog.String("version", version.String()),
		slog.String("event.name", eventType),
	)

	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		event.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}

	return &eventBuilder{r: event}
}

func (e *eventBuilder) Context(attrs ...attribute.KeyValue) *eventBuilder {
	if e != nil {
		e.r.AddAttrs(slog.GroupAttrs("context", attrsToSlog(attrs...)...))
	}

	return e
}

func (e *eventBuilder) Msgf(format string, v ...any) { e.Msg(fmt.Sprintf(format, v...)) }
func (e *eventBuilder) Msg(msg string) {
	e.r.Message = msg
	e.h.Handle(e.ctx, e.r)
}

func omitOK[T any](s T, ok bool) T {
	if !ok {
		return *new(T)
	}
	return s
}

func attrsToSlog(attrs ...attribute.KeyValue) []slog.Attr {
	res := make([]slog.Attr, len(attrs))
	for i, attr := range attrs {
		res[i] = slog.Attr{
			Key:   string(attr.Key),
			Value: valueSlog(attr.Value),
		}
	}
	return res
}

func valueSlog(attrs attribute.Value) slog.Value {
	switch attrs.Type() {
	case attribute.BOOL:
		return slog.BoolValue(attrs.AsBool())
	case attribute.BOOLSLICE:
		return slog.AnyValue(attrs.AsBoolSlice())
	case attribute.FLOAT64:
		return slog.Float64Value(attrs.AsFloat64())
	case attribute.FLOAT64SLICE:
		return slog.AnyValue(attrs.AsFloat64Slice())
	case attribute.INT64:
		return slog.Int64Value(attrs.AsInt64())
	case attribute.INT64SLICE:
		return slog.AnyValue(attrs.AsInt64Slice())
	case attribute.STRING:
		return slog.StringValue(attrs.AsString())
	case attribute.STRINGSLICE:
		return slog.AnyValue(attrs.AsStringSlice())
	default:
		return slog.AnyValue(attrs)
	}
}

func asEnvs(envs map[string]string) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, len(envs))
	for k, v := range envs {
		attrs = append(attrs, semconv.ProcessEnvironmentVariable(k, v))
	}
	return attrs
}

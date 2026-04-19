package chatmodel

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	semconv "go.opentelemetry.io/otel/semconv/v1.39.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/chatmodel/genai"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
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

type streamCallback interface {
	span
	addOutputMessage(msg messages.Message)
}

type streamSpan struct {
	spanCallback
	current   *genai.ChatMessage
	reason    genai.FinishReason
	collected []genai.ChatMessage
}

//nolint:ireturn // it's a factory function
func (o *observable) stream(
	ctx context.Context,
	input []messages.Message,
	settings entities.AgentReadOnly,
) (context.Context, streamCallback) {
	attrs := []attribute.KeyValue{
		semconv.GenAIOperationNameGenerateContent,
		// TODO: here is the catch: i have no clue how to correctly set
		// provider from fields. For now there is Gemini only, but for
		// later... Use model with provider? no idea...
		semconv.GenAIProviderNameGCPGemini,
		semconv.GenAIRequestModel(settings.Model()),
		marshalMessagesToGenai(settings.SystemMessage(), input),
	}

	if v, ok := settings.TopP(); ok {
		attrs = append(attrs, semconv.GenAIRequestTopP(float64(v)))
	}

	if v, ok := settings.Temperature(); ok {
		attrs = append(attrs, semconv.GenAIRequestTemperature(float64(v)))
	}

	//nolint:spancheck // safe to omit, since wrapper automatically closes span.
	ctx, span := o.t.Start(ctx, "cynosure.ports.chatmodel.stream",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(attrs...),
	)

	//nolint:spancheck // see above
	return ctx, &streamSpan{
		spanCallback: spanCallback{span: span},
		collected:    nil,
		current:      nil,
		reason:       "",
	}
}

func (s *streamSpan) addOutputMessage(msg messages.Message) {
	var err error

	s.current, err = concatenateMessage(&s.collected, s.current, msg)
	if err != nil {
		s.recordError(err)
		s.reason = genai.FinishReasonError
	}
}

func (s *streamSpan) end() {
	if s.current != nil {
		s.collected = append(s.collected, *s.current)
	}

	if s.reason == "" {
		s.reason = genai.FinishReasonStop
	}

	if len(s.collected) > 0 {
		s.span.SetAttributes(marshalResponseToGenai(s.collected, s.reason))
	}

	s.spanCallback.end()
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

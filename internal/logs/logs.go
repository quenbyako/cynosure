package logs

import (
	"context"
	"log/slog"
	"net"

	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.39.0"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

const (
	eventMaxTurnsReached      = "generate.max_turns_reached"
	eventToolCalled           = "generate.tool_called"
	eventGeminiStreamStarted  = "gemini.stream_started"
	eventEffectiveEnvironment = "notify.effective_environment"
	eventMetricsStarted       = "metrics.started"
	eventMetricsStopped       = "metrics.stopped"
)

const (
	TelegramChannelID attribute.Key = "telegram.channel_id"
)

// telegram.LogCallbacks

func (l *BaseLogger) ProcessMessageIssue(ctx context.Context, channelID int, err error) {
	l.event(ctx, slog.LevelError, "message.issue").
		Context(
			TelegramChannelID.Int(channelID),
			semconv.ErrorTypeKey.String(err.Error()),
		).
		Msg("message issue")
}

func (l *BaseLogger) ProcessMessageStart(ctx context.Context, channelID int, messageText string) {
	l.l.Info().Int("channel_id", channelID).Str("message_text", messageText).Msg("message start")
}

func (l *BaseLogger) ProcessMessageSuccess(ctx context.Context, channelID int, duration string) {
	l.l.Info().Int("channel_id", channelID).Str("duration", duration).Msg("message success")
}

func (l *BaseLogger) MaxTurnsReached(ctx context.Context, threadID string) {
	l.l.Warn().
		Str("event_type", eventMaxTurnsReached).
		Any("context",
			map[string]any{
				"thread_id": threadID,
			},
		).
		Msg("Model reached max turns with tool calls, consider adjusting settings")
}

func (l *BaseLogger) ToolCalled(ctx context.Context, threadID string, toolRequests []messages.MessageToolRequest) {
	names := make([]string, len(toolRequests))
	for i, req := range toolRequests {
		names[i] = req.ToolName()
	}

	l.l.Info().
		Str("event_type", eventToolCalled).
		Any("context",
			map[string]any{
				"thread_id":  threadID,
				"tool_names": names,
			},
		).
		Msg("Tool called during generation")
}

func (l *BaseLogger) EffectiveEnvironment(env map[string]string) {
	l.l.Info().
		Str("event_type", eventEffectiveEnvironment).
		Any("context",
			map[string]any{
				"env": env,
			},
		).
		Msg("Parsed effective environment")
}

func (l *BaseLogger) MetricsStarted(addr net.Addr) {
	l.l.Info().
		Str("event_type", eventMetricsStarted).
		Any("context",
			map[string]any{
				"addr": addr.String(),
			},
		).
		Msg("Metrics server started")
}

func (l *BaseLogger) MetricsStopped(addr net.Addr) {
	l.l.Info().
		Str("event_type", eventMetricsStopped).
		Any("context",
			map[string]any{
				"addr": addr.String(),
			},
		).
		Msg("Metrics server stopped")
}

func (l *BaseLogger) GeminiStreamStarted(ctx context.Context, model string, toolCount int) {
	l.l.Info().
		Str("event_type", eventGeminiStreamStarted).
		Any("context",
			map[string]any{
				"model":      model,
				"tool_count": toolCount,
			},
		).
		Msg("Passing tools to Gemini")
}

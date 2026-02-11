package logs

import (
	"context"
	"net"

	"github.com/quenbyako/core/contrib/runtime"
	"github.com/quenbyako/cynosure/contrib/onelog"

	"github.com/quenbyako/cynosure/internal/adapters/gemini"
	"github.com/quenbyako/cynosure/internal/controllers/telegram"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/chat"
)

func New(l onelog.Logger) *BaseLogger { return &BaseLogger{l: l} }

type BaseLogger struct {
	l onelog.Logger
}

var _ chat.LogCallbacks = (*BaseLogger)(nil)
var _ gemini.LogCallbacks = (*BaseLogger)(nil)
var _ telegram.LogCallbacks = (*BaseLogger)(nil)
var _ runtime.LogCallbacks = (*BaseLogger)(nil)

const (
	eventMaxTurnsReached      = "generate.max_turns_reached"
	eventToolCalled           = "generate.tool_called"
	eventGeminiStreamStarted  = "gemini.stream_started"
	eventEffectiveEnvironment = "notify.effective_environment"
	eventMetricsStarted       = "metrics.started"
	eventMetricsStopped       = "metrics.stopped"
)

// telegram.LogCallbacks

func (l *BaseLogger) ProcessMessageIssue(ctx context.Context, channelID int, err error) {
	l.l.Error().Err(err).Int("channel_id", channelID).Msg("message issue")
}

func (l *BaseLogger) ProcessMessageStart(ctx context.Context, channelID int, messageText string) {
	l.l.Info().Int("channel_id", channelID).Str("message_text", messageText).Msg("message start")
}

func (l *BaseLogger) ProcessMessageSuccess(ctx context.Context, channelID int, duration string) {
	l.l.Info().Int("channel_id", channelID).Str("duration", duration).Msg("message success")
}

func (l *BaseLogger) MaxTurnsReached(ctx context.Context, threadID ids.ThreadID) {
	l.l.Warn().
		Str("event_type", eventMaxTurnsReached).
		Any("context",
			map[string]any{
				"thread_id": threadID.String(),
			},
		).
		Msg("Model reached max turns with tool calls, consider adjusting settings")
}

func (l *BaseLogger) ToolCalled(ctx context.Context, threadID ids.ThreadID, toolRequests []messages.MessageToolRequest) {
	names := make([]string, len(toolRequests))
	for i, req := range toolRequests {
		names[i] = req.ToolName()
	}

	l.l.Info().
		Str("event_type", eventToolCalled).
		Any("context",
			map[string]any{
				"thread_id":  threadID.String(),
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

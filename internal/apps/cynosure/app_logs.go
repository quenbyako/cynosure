package cynosure

import (
	"context"

	goose "github.com/quenbyako/cynosure/contrib/mongoose"
	"github.com/quenbyako/cynosure/contrib/onelog"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/messages"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/chat"
)

const (
	eventMaxTurnsReached      = "generate.max_turns_reached"
	eventToolCalled           = "generate.tool_called"
	eventEffectiveEnvironment = "notify.effective_environment"
)

type logger struct {
	log onelog.Logger
}

var _ chat.LogCallbacks = (*logger)(nil)
var _ goose.LogCallbacks = (*logger)(nil)

func newLogCallbacks(p *appParams) *logger {
	return &logger{log: onelog.Wrap(p.log)}
}

func (l *logger) MaxTurnsReached(ctx context.Context, threadID, userID string) {
	l.log.Warn().
		Str("event_type", eventMaxTurnsReached).
		Any("context",
			map[string]any{
				"thread_id": threadID,
				"user_id":   userID,
			},
		).
		Msg("Model reached max turns with tool calls, consider adjusting settings")
}

func (l *logger) ToolCalled(ctx context.Context, threadID, userID string, toolRequests []messages.MessageToolRequest) {
	l.log.Info().
		Str("event_type", eventToolCalled).
		Any("context",
			map[string]any{
				"thread_id":  threadID,
				"user_id":    userID,
				"tool_names": toolRequests,
			},
		).
		Msg("Tool called during generation")
}

func (l *logger) EffectiveEnvironment(env map[string]string) {
	l.log.Info().
		Str("event_type", eventEffectiveEnvironment).
		Any("context",
			map[string]any{
				"env": env,
			},
		).
		Msg("Parsed effective environment")
}

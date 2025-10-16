package goose

import "github.com/quenbyako/cynosure/contrib/onelog"

const (
	eventEffectiveEnvironment = "notify.effective_environment"
)

type LogCallbacks interface {
	EffectiveEnvironment(env map[string]string)
}

type logger struct {
	log onelog.Logger
}

var _ LogCallbacks = (*logger)(nil)

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

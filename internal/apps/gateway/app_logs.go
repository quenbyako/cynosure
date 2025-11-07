package gateway

import (
	"context"
	"net"

	"github.com/google/wire"
	"github.com/quenbyako/core/contrib/runtime"
	"github.com/quenbyako/cynosure/contrib/onelog"

	"github.com/quenbyako/cynosure/internal/controllers/tgbot"
)

var (
	loggerConstructor = wire.NewSet(
		newLogCallbacks,
		wire.Bind(new(tgbot.LogCallbacks), new(*logger)),
	)
)

const (
	eventProcessingMessage    = "tgbot.processing_message"
	eventEffectiveEnvironment = "notify.effective_environment"
	eventMetricsStarted       = "metrics.started"
	eventMetricsStopped       = "metrics.stopped"
)

type logger struct {
	log onelog.Logger
}

var _ tgbot.LogCallbacks = (*logger)(nil)
var _ runtime.LogCallbacks = (*logger)(nil)

func newLogCallbacks(p *appParams) *logger {
	return &logger{log: onelog.Wrap(p.observability)}
}

func (l *logger) ProcessMessageIssue(ctx context.Context, channelID int64, err error) {
	l.log.Error().
		Str("event_type", eventProcessingMessage).
		Any("context",
			map[string]any{
				"channel_id": channelID,
				"error":      err.Error(),
			},
		).
		Msg("Can't consume message for chat")
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

func (l *logger) MetricsStarted(addr net.Addr) {
	l.log.Info().
		Str("event_type", eventMetricsStarted).
		Any("context",
			map[string]any{
				"addr": addr.String(),
			},
		).
		Msg("Metrics server started")
}

func (l *logger) MetricsStopped(addr net.Addr) {
	l.log.Info().
		Str("event_type", eventMetricsStopped).
		Any("context",
			map[string]any{
				"addr": addr.String(),
			},
		).
		Msg("Metrics server stopped")
}

// Package logs provides logging utilities.
package logs

import (
	"context"
	"log/slog"
	"net"

	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
)

const (
	eventGeminiStreamStarted  = "gemini.stream_started"
	eventEffectiveEnvironment = "notify.effective_environment"
	eventMetricsStarted       = "metrics.started"
	eventMetricsStopped       = "metrics.stopped"
)

const (
	TelegramChannelID attribute.Key = "telegram.channel_id"
)

// telegram.LogCallbacks

func (l *BaseLogger) TelegramPoolNotRunning(ctx context.Context) {
	l.event(ctx, slog.LevelError, "controller.telegram.pool_not_running").
		Msg("work pool not running")
}

func (l *BaseLogger) ProcessMessageIssue(ctx context.Context, channelID int, err error) {
	l.event(ctx, slog.LevelError, "message.issue").
		Context(
			TelegramChannelID.Int(channelID),
			semconv.ErrorTypeKey.String(err.Error()),
		).
		Msg("message issue")
}

func (l *BaseLogger) ProcessMessageStart(ctx context.Context, channelID int, messageText string) {
	l.event(ctx, slog.LevelInfo, "message.start").
		Context(
			TelegramChannelID.Int(channelID),
			attribute.Key("message.text").String(messageText),
		).
		Msg("message start")
}

func (l *BaseLogger) ProcessMessageSuccess(ctx context.Context, channelID int, duration string) {
	l.event(ctx, slog.LevelInfo, "message.success").
		Context(
			TelegramChannelID.Int(channelID),
			attribute.Key("duration").String(duration),
		).
		Msg("message success")
}

func (l *BaseLogger) EffectiveEnvironment(ctx context.Context, env map[string]string) {
	l.event(ctx, slog.LevelInfo, eventEffectiveEnvironment).
		Context(
			asEnvs(env)...,
		).
		Msg("Parsed effective environment")
}

func (l *BaseLogger) MetricsStarted(ctx context.Context, addr net.Addr) {
	l.event(ctx, slog.LevelInfo, eventMetricsStarted).
		Context(
			attribute.Key("addr").String(addr.String()),
		).
		Msg("Metrics server started")
}

func (l *BaseLogger) MetricsStopped(ctx context.Context, addr net.Addr) {
	l.event(ctx, slog.LevelInfo, eventMetricsStopped).
		Context(
			attribute.Key("addr").String(addr.String()),
		).
		Msg("Metrics server stopped")
}

func (l *BaseLogger) GeminiStreamStarted(ctx context.Context, model string, toolCount int) {
	l.event(ctx, slog.LevelInfo, eventGeminiStreamStarted).
		Context(
			attribute.Key("model").String(model),
			attribute.Key("tool_count").Int(toolCount),
		).
		Msg("Passing tools to Gemini")
}

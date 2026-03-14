package mcp

import (
	"context"
	"log/slog"
	"time"
)

// TODO: make correct log callbacks, this shit makes absolutely no sense
func logWarn(ctx context.Context, logger slog.Handler, msg string, args ...any) {
	if logger != nil && logger.Enabled(ctx, slog.LevelWarn) {
		r := slog.NewRecord(time.Now(), slog.LevelWarn, msg, 0)
		r.Add(args...)
		//nolint:errcheck // we don't care about logger errors
		_ = logger.Handle(ctx, r)
	}
}

func logError(ctx context.Context, logger slog.Handler, msg string, args ...any) {
	if logger != nil && logger.Enabled(ctx, slog.LevelError) {
		r := slog.NewRecord(time.Now(), slog.LevelError, msg, 0)
		r.Add(args...)
		//nolint:errcheck // we don't care about logger errors
		_ = logger.Handle(ctx, r)
	}
}

func logDebug(ctx context.Context, logger slog.Handler, msg string, args ...any) {
	if logger != nil && logger.Enabled(ctx, slog.LevelDebug) {
		r := slog.NewRecord(time.Now(), slog.LevelDebug, msg, 0)
		r.Add(args...)
		//nolint:errcheck // we don't care about logger errors
		_ = logger.Handle(ctx, r)
	}
}

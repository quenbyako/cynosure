package cynosure

import (
	"go.opentelemetry.io/contrib/bridges/otelslog"

	"github.com/quenbyako/cynosure/internal/logs"
)

func newLogger(p *appParams) *logs.BaseLogger {
	// TODO: remove slog bridge, and generate plain otel records.
	return logs.New(otelslog.NewHandler("cynosure", otelslog.WithLoggerProvider(p.observability)))
}

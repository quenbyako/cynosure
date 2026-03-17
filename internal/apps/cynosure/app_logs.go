package cynosure

import (
	"github.com/quenbyako/cynosure/internal/logs"
)

func newLogger(p *appParams) *logs.BaseLogger {
	return logs.New(p.observability)
}

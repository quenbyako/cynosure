package cynosure

import (
	"github.com/goforj/wire"
	"github.com/quenbyako/core/contrib/runtime"

	"github.com/quenbyako/cynosure/internal/adapters/gemini"
	"github.com/quenbyako/cynosure/internal/controllers/telegram"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/chat"
	"github.com/quenbyako/cynosure/internal/logs"
)

var loggerConstructor = wire.NewSet(
	newLogger,
	wire.Bind(new(chat.LogCallbacks), new(*logs.BaseLogger)),
	wire.Bind(new(gemini.LogCallbacks), new(*logs.BaseLogger)),
	wire.Bind(new(telegram.LogCallbacks), new(*logs.BaseLogger)),
	wire.Bind(new(runtime.LogCallbacks), new(*logs.BaseLogger)),
)

func newLogger(p *appParams) *logs.BaseLogger {
	return logs.New(p.observability)
}

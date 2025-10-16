//go:build wireinject

package cynosure

import (
	"context"

	"github.com/google/wire"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/chat"
)

var (
	loggerConstructor = wire.NewSet(
		newLogCallbacks,
		wire.Bind(new(chat.LogCallbacks), new(*logger)),
	)
)

func buildApp(ctx context.Context, config *appParams) (*App, error) {
	panic(wire.Build(
		ports.WirePorts,

		loggerConstructor,

		fileAdapter,
		zepAdapter,
		geminiAdapter,
		primitiveAdapter,
		oauthAdapter,

		chatUsecase,
		accountsUsecase,
		serversUsecase,

		newApp,
	))
}

//go:build wireinject

package cynosure

import (
	"context"

	"github.com/goforj/wire"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/oauthhandler"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/toolclient"
)

func buildApp(ctx context.Context, config *appParams) (*App, error) {
	panic(wire.Build(
		ports.WirePorts,
		toolclient.New,
		oauthhandler.New,

		loggerConstructor,

		sqlAdapter,
		geminiAdapter,
		mcpAdapter,
		oauthAdapter,
		oryAdapter,

		chatUsecase,
		accountsUsecase,
		usersUsecase,

		connectDependencies,
	))
}

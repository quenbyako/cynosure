//go:build wireinject

package gateway

import (
	"context"

	"github.com/google/wire"

	"github.com/quenbyako/cynosure/internal/domains/gateway/ports"
)

func buildApp(ctx context.Context, config *appParams) (*App, error) {
	panic(wire.Build(
		ports.WirePorts,

		telegramAdapter,

		mainUsecase,

		newApp,
	))
}

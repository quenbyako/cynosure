package gateway

import (
	"context"

	"github.com/google/wire"

	"github.com/quenbyako/cynosure/internal/domains/gateway/ports"
	"github.com/quenbyako/cynosure/internal/domains/gateway/usecases"
)

var (
	mainUsecase = wire.NewSet(newUsecase)
)

func newUsecase(
	ctx context.Context,
	p *appParams,
	messenger ports.Messenger,
	a2a ports.Agent,
) (*usecases.Usecase, error) {
	return usecases.NewUsecase(messenger, a2a), nil
}

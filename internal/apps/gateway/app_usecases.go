package gateway

import (
	"context"

	"github.com/google/wire"

	"github.com/quenbyako/cynosure/internal/domains/gateway/usecases"
)

var (
	mainUsecase = wire.NewSet(newUsecase)
)

func newUsecase(
	ctx context.Context,
	p *appParams,
) (*usecases.Usecase, error) {
	return &usecases.Usecase{}, nil
}

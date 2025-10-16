package gateway

import (
	"context"

	"github.com/google/wire"

	"github.com/quenbyako/cynosure/internal/adapters/telegram"
	"github.com/quenbyako/cynosure/internal/domains/gateway/ports"
)

var (
	telegramAdapter = wire.NewSet(
		newTelegramAdapter,
		wire.Bind(new(ports.MessengerFactory), new(*telegram.Messenger)),
	)
)

func newTelegramAdapter(ctx context.Context, p *appParams) (*telegram.Messenger, error) {
	return &telegram.Messenger{}, nil
}

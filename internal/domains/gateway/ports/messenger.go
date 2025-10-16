package ports

import (
	"context"

	"github.com/quenbyako/cynosure/internal/domains/gateway/components"
	"github.com/quenbyako/cynosure/internal/domains/gateway/components/ids"
)

type Messenger interface {
	SendMessage(ctx context.Context, channelID ids.ChannelID, text chan components.MessageText) error
}

type MessengerFactory interface {
	Messenger() Messenger
}

func NewMessenger(factory MessengerFactory) Messenger { return factory.Messenger() }

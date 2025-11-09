package ports

import (
	"context"

	"github.com/quenbyako/cynosure/internal/domains/gateway/components"
	"github.com/quenbyako/cynosure/internal/domains/gateway/components/ids"
)

type Messenger interface {
	SendMessage(ctx context.Context, channelID ids.ChannelID, text components.MessageText) (ids.MessageID, error)
	UpdateMessage(ctx context.Context, messageID ids.MessageID, text components.MessageText) error
	// should be called, when the message is received and is being processed now
	NotifyProcessingStarted(ctx context.Context, channelID ids.ChannelID) error
}

type MessengerFactory interface {
	Messenger() Messenger
}

func NewMessenger(factory MessengerFactory) Messenger { return factory.Messenger() }

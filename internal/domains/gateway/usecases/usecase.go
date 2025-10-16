package usecases

import (
	"context"
	"fmt"

	"github.com/quenbyako/cynosure/internal/domains/gateway/entities"
	"github.com/quenbyako/cynosure/internal/domains/gateway/ports"
)

type Usecase struct {
	client ports.Messenger
	a2a    ports.Agent
}

func (u *Usecase) ReceiveNewMessageEvent(ctx context.Context, msg *entities.Message) error {
	text, ok := msg.Text()
	if !ok {
		return nil // Игнорируем сообщения без текста
	}

	resp, err := u.a2a.SendMessage(ctx, msg.ID().ChannelID(), text)
	if err != nil {
		return fmt.Errorf("failed to process message via a2a: %w", err)
	}

	return u.client.SendMessage(ctx, msg.ID().ChannelID(), resp)
}

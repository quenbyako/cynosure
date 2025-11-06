package usecases

import (
	"context"
	"fmt"
	"sync"

	"github.com/quenbyako/cynosure/internal/domains/gateway/components"
	"github.com/quenbyako/cynosure/internal/domains/gateway/entities"
	"github.com/quenbyako/cynosure/internal/domains/gateway/ports"
)

type Usecase struct {
	client ports.Messenger
	a2a    ports.Agent
}

func NewUsecase(
	client ports.Messenger,
	a2a ports.Agent,
) *Usecase {
	if client == nil {
		panic("messenger client is required")
	}
	if a2a == nil {
		panic("agent-to-agent component is required")
	}

	return &Usecase{
		client: client,
		a2a:    a2a,
	}
}

func (u *Usecase) ReceiveNewMessageEvent(ctx context.Context, msg *entities.Message) error {
	text, ok := msg.Text()
	if !ok {
		return nil // Игнорируем сообщения без текста
	}

	if err := u.client.NotifyProcessingStarted(ctx, msg.ID().ChannelID()); err != nil {
		return fmt.Errorf("notify processing started: %w", err)
	}

	resp, err := u.a2a.SendMessage(ctx, msg.ID(), text)
	if err != nil {
		return fmt.Errorf("failed to process message via a2a: %w", err)
	}

	textChan := make(chan components.MessageText)
	var wg sync.WaitGroup
	wg.Go(func() {
		for part, err := range resp {
			if err != nil {
				txt, err := components.NewMessageText(fmt.Sprintf("<Error: %v>", err))
				if err != nil {
					// TODO: log
					continue
				}
				textChan <- txt
				continue
			}

			textChan <- part
		}
	})

	if err := u.client.SendMessage(ctx, msg.ID().ChannelID(), textChan); err != nil {
		return fmt.Errorf("sending message via messenger: %w", err)
	}
	wg.Wait()

	return nil
}

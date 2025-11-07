package usecases

import (
	"context"
	"fmt"

	"github.com/quenbyako/cynosure/internal/domains/gateway/components"
	"github.com/quenbyako/cynosure/internal/domains/gateway/components/ids"
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

	// Accumulate full text and update message with complete version
	var sentMessageID ids.MessageID
	var accumulated string

	// Consume streaming response and update with full accumulated text
	for part, err := range resp {
		if err != nil {
			// TODO: Send error to user using userFriendlyError()
			return fmt.Errorf("streaming response error: %w", err)
		}

		// Accumulate the text
		accumulated += part.Text()

		// Create MessageText with full accumulated content
		fullText, err := components.NewMessageText(accumulated)
		if err != nil {
			return fmt.Errorf("creating accumulated message text: %w", err)
		}

		if !sentMessageID.Valid() {
			// Send initial message with first chunk
			sentMessageID, err = u.client.SendMessage(ctx, msg.ID().ChannelID(), fullText)
			if err != nil {
				return fmt.Errorf("sending initial message via messenger: %w", err)
			}
		} else {
			// Update message with full accumulated text
			if err := u.client.UpdateMessage(ctx, sentMessageID, fullText); err != nil {
				return fmt.Errorf("updating message via messenger: %w", err)
			}
		}
	}

	return nil
}

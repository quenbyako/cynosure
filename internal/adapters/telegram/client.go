package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/quenbyako/cynosure/internal/domains/gateway/components"
	"github.com/quenbyako/cynosure/internal/domains/gateway/components/ids"
	"github.com/quenbyako/cynosure/internal/domains/gateway/ports"
)

type Messenger struct {
	api *tgbotapi.BotAPI

	user tgbotapi.User
}

var _ ports.MessengerFactory = (*Messenger)(nil)

func (m *Messenger) Messenger() ports.Messenger { return m }

type NewMessengerOption func(*newMessengerParams)

type newMessengerParams struct {
	webhookParams *tgbotapi.WebhookConfig
}

func WithWebhook(webhookParams tgbotapi.WebhookConfig) NewMessengerOption {
	return func(p *newMessengerParams) { p.webhookParams = &webhookParams }
}

func NewMessenger(apiToken string, opts ...NewMessengerOption) (*Messenger, error) {
	p := newMessengerParams{
		webhookParams: nil,
	}
	for _, opt := range opts {
		opt(&p)
	}

	api, err := tgbotapi.NewBotAPI(apiToken)
	if err != nil {
		return nil, err
	}
	user, err := api.GetMe()
	if err != nil {
		return nil, err
	}

	if p.webhookParams != nil {
		if _, err := api.Request(p.webhookParams); err != nil {
			return nil, fmt.Errorf("failed to set webhook: %w", err)
		}
	}

	return &Messenger{
		api:  api,
		user: user,
	}, nil
}

func (m *Messenger) SendMessage(_ context.Context, channelID ids.ChannelID, text chan components.MessageText) error {
	if !channelID.Valid() {
		return fmt.Errorf("invalid channel id")
	}

	if provider := channelID.ProviderID(); provider != "telegram" {
		return fmt.Errorf("unsupported provider %q, expected %q", provider, "telegram")
	}

	chatID, err := strconv.ParseInt(channelID.ChannelID(), 10, 64)
	if err != nil {
		return err
	}

	var message string
	for data := range text {
		message += data.Text()
	}

	if message = strings.TrimSpace(message); message == "" {
		return fmt.Errorf("can't send empty message")
	}

	_, err = m.api.Send(tgbotapi.NewMessage(chatID, message))

	return err
}

func (m *Messenger) NotifyProcessingStarted(ctx context.Context, channelID ids.ChannelID) error {
	if provider := channelID.ProviderID(); provider != "telegram" {
		return fmt.Errorf("unsupported provider %q, expected %q", provider, "telegram")
	}

	chatID, err := strconv.ParseInt(channelID.ChannelID(), 10, 64)
	if err != nil {
		return err
	}

	_, err = m.api.Request(tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping))
	return err

}

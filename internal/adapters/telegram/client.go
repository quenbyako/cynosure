package telegram

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	tgbotapi "github.com/quenbyako/cynosure/contrib/telegram-bot-api/v9"

	"github.com/quenbyako/cynosure/internal/domains/gateway/components"
	"github.com/quenbyako/cynosure/internal/domains/gateway/components/ids"
	"github.com/quenbyako/cynosure/internal/domains/gateway/ports"
)

// TODO: add a DTO for Telegram types. For example, maximum message length is
// not standardized here and not correlated to business logic at all.
type Messenger struct {
	api *tgbotapi.BotAPI
}

var _ ports.MessengerFactory = (*Messenger)(nil)

func (m *Messenger) Messenger() ports.Messenger { return m }

type NewMessengerOption func(*newMessengerParams)

type newMessengerParams struct {
	webhookParams *tgbotapi.WebhookConfig
	endpoint      string
	client        http.RoundTripper
}

func WithWebhook(webhookParams tgbotapi.WebhookConfig) NewMessengerOption {
	return func(p *newMessengerParams) { p.webhookParams = &webhookParams }
}

func WithRoundTripper(rt http.RoundTripper) NewMessengerOption {
	return func(p *newMessengerParams) { p.client = rt }
}

func WithEndpoint(endpoint string) NewMessengerOption {
	return func(p *newMessengerParams) { p.endpoint = endpoint }
}

func NewMessenger(ctx context.Context, apiToken string, opts ...NewMessengerOption) (*Messenger, error) {
	p := newMessengerParams{
		webhookParams: nil,
		endpoint:      tgbotapi.APIEndpoint,
		client:        http.DefaultTransport,
	}
	for _, opt := range opts {
		opt(&p)
	}

	api, err := tgbotapi.NewBotAPIWithClient(ctx, apiToken, p.endpoint, &http.Client{
		Transport: p.client,
	})
	if err != nil {
		return nil, fmt.Errorf("creating a client: %w", err)
	}

	if p.webhookParams != nil {
		if _, err := api.Request(ctx, p.webhookParams); err != nil {
			return nil, fmt.Errorf("setting webhook: %w", err)
		}
	}

	return &Messenger{
		api: api,
	}, nil
}

// SendMessage sends an initial message and returns the message ID for future appends
func (m *Messenger) SendMessage(ctx context.Context, channelID ids.ChannelID, text components.MessageText) (ids.MessageID, error) {
	if !channelID.Valid() {
		return ids.MessageID{}, fmt.Errorf("invalid channel id")
	}

	if provider := channelID.ProviderID(); provider != "telegram" {
		return ids.MessageID{}, fmt.Errorf("unsupported provider %q, expected %q", provider, "telegram")
	}

	chatID, err := strconv.ParseInt(channelID.ChannelID(), 10, 64)
	if err != nil {
		return ids.MessageID{}, err
	}

	content := strings.TrimSpace(text.Text())
	if content == "" {
		return ids.MessageID{}, fmt.Errorf("cannot send empty message")
	}

	// Truncate if too long (Telegram limit is 4096)
	const maxMessageLength = 4080
	if len(content) > maxMessageLength {
		content = content[:maxMessageLength] + "...[truncated]"
	}

	msg := tgbotapi.NewMessage(chatID, content)
	sent, err := m.api.Send(ctx, msg)
	if err != nil {
		return ids.MessageID{}, fmt.Errorf("send message: %w", err)
	}

	// Create message ID from Telegram message ID
	messageID, err := ids.NewMessageID(
		channelID,
		fmt.Sprintf("%d", sent.MessageID),
	)
	if err != nil {
		return ids.MessageID{}, fmt.Errorf("create message id: %w", err)
	}

	return messageID, nil
}

// UpdateMessage updates text in an existing message by editing it
func (m *Messenger) UpdateMessage(ctx context.Context, messageID ids.MessageID, text components.MessageText) error {
	if !messageID.Valid() {
		return fmt.Errorf("invalid message id")
	}

	channelID := messageID.ChannelID()
	if provider := channelID.ProviderID(); provider != "telegram" {
		return fmt.Errorf("unsupported provider %q, expected %q", provider, "telegram")
	}

	chatID, err := strconv.ParseInt(channelID.ChannelID(), 10, 64)
	if err != nil {
		return err
	}

	tgMessageID, err := strconv.Atoi(messageID.MessageID())
	if err != nil {
		return fmt.Errorf("invalid telegram message id: %w", err)
	}

	content := strings.TrimSpace(text.Text())
	if content == "" {
		return nil // Nothing to append
	}

	// Truncate if too long (Telegram limit is 4096)
	const maxMessageLength = 4080
	if len(content) > maxMessageLength {
		content = content[:maxMessageLength] + "...[truncated]"
	}

	edit := tgbotapi.NewEditMessageText(chatID, tgMessageID, content)
	_, err = m.api.Send(ctx, edit)
	if err != nil {
		// Ignore "message is not modified" errors
		if strings.Contains(err.Error(), "message is not modified") {
			return nil
		}
		return fmt.Errorf("edit message: %w", err)
	}

	return nil
}

func (m *Messenger) NotifyProcessingStarted(ctx context.Context, channelID ids.ChannelID) error {
	if provider := channelID.ProviderID(); provider != "telegram" {
		return fmt.Errorf("unsupported provider %q, expected %q", provider, "telegram")
	}

	chatID, err := strconv.ParseInt(channelID.ChannelID(), 10, 64)
	if err != nil {
		return err
	}

	_, err = m.api.Request(ctx, tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping))
	return err

}

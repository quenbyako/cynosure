package telegram

import (
	"context"
	"fmt"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/k0kubun/pp/v3"

	"github.com/quenbyako/cynosure/internal/domains/gateway/components"
	"github.com/quenbyako/cynosure/internal/domains/gateway/components/ids"
	"github.com/quenbyako/cynosure/internal/domains/gateway/ports"
)

type Messenger struct {
	api *tgbotapi.BotAPI
}

var _ ports.MessengerFactory = (*Messenger)(nil)

func (m *Messenger) Messenger() ports.Messenger { return m }

func (m *Messenger) SendMessage(_ context.Context, channelID ids.ChannelID, text chan components.MessageText) error {
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
	msg, err := m.api.Send(tgbotapi.NewMessage(chatID, message))
	pp.Println(msg)

	return err
}

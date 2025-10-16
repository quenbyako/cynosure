package tgbot

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/quenbyako/cynosure/internal/domains/gateway/components"
	"github.com/quenbyako/cynosure/internal/domains/gateway/components/ids"
	"github.com/quenbyako/cynosure/internal/domains/gateway/entities"
	"github.com/quenbyako/cynosure/internal/domains/gateway/usecases"
)

type Client struct {
	bot  *tgbotapi.BotAPI
	chat *usecases.Usecase

	wg sync.WaitGroup
}

func New() *Client {
	bot, err := tgbotapi.NewBotAPI("MyAwesomeBotToken")
	if err != nil {
		panic(err)
	}
	bot.Debug = true

	return &Client{
		bot: bot,
	}
}

func (c *Client) Close() error {
	c.bot.StopReceivingUpdates()
	c.wg.Wait()
	return nil
}

func (c *Client) Run(ctx context.Context) error {
	c.wg.Add(1)
	defer c.wg.Done()

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	for upd := range c.bot.GetUpdatesChan(u) {
		c.process(ctx, &upd)
	}

	return nil
}

func (c *Client) process(ctx context.Context, upd *tgbotapi.Update) {
	switch {
	case upd.Message != nil:
		c.processMessage(ctx, upd.Message)
	default:
		fmt.Println("warn: unhandled update")
	}
}

func (c *Client) processMessage(ctx context.Context, msg *tgbotapi.Message) {
	if msg.Text == "" {
		return // Игнорируем пустые сообщения
	}

	// Отправляем "typing..."
	_, _ = c.bot.Send(tgbotapi.NewChatAction(msg.Chat.ID, tgbotapi.ChatTyping))

	channelID, err := ids.NewChannelID("telegram", strconv.FormatInt(msg.Chat.ID, 10))
	if err != nil {
		panic(err)
	}

	msgID, err := ids.NewMessageID(channelID, strconv.Itoa(msg.MessageID))
	if err != nil {
		panic(err)
	}
	authorID, err := ids.NewUserID("telegram", strconv.FormatInt(msg.From.ID, 10))
	if err != nil {
		panic(err)
	}

	text, err := components.NewMessageText(msg.Text)
	if err != nil {
		panic(err)
	}

	message, err := entities.NewMessage(msgID, authorID, entities.WithText(text))
	if err != nil {
		panic(err)
	}

	if err := c.chat.ReceiveNewMessageEvent(ctx, message); err != nil {
		panic(err)
	}
}

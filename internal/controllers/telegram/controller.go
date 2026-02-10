package tgbot

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	botapi "github.com/quenbyako/cynosure/contrib/tg-openapi/gen/go/botapi"

	"github.com/quenbyako/cynosure/internal/domains/gateway/components"
	"github.com/quenbyako/cynosure/internal/domains/gateway/components/ids"
	"github.com/quenbyako/cynosure/internal/domains/gateway/entities"
	"github.com/quenbyako/cynosure/internal/domains/gateway/usecases"
)

type Handler struct {
	log LogCallbacks

	srv *usecases.Usecase
}

var _ botapi.StrictWebhookInterface = (*Handler)(nil)

func NewHandler(logs LogCallbacks, srv *usecases.Usecase) http.Handler {
	h := &Handler{
		srv: srv,
	}

	inner := botapi.NewStrictWebhookHandler(h, []botapi.StrictMiddlewareFunc{})

	return botapi.WebhookHandler(inner)
}

func (h *Handler) SendUpdate(ctx context.Context, request botapi.SendUpdateRequestObject) (botapi.SendUpdateResponseObject, error) {
	update := request.Body
	if update == nil {
		return nil, fmt.Errorf("update is nil")
	}

	updateID := update.UpdateId
	switch {
	case update.Message != nil:
		if res, err := h.processMessage(ctx, updateID, update.Message); err != nil {
			return nil, err
		} else {
			return res, nil
		}
	default:
		// Unknown update type, ignore
		return botapi.SendUpdate204Response{}, nil
	}

}

func (h *Handler) processMessage(ctx context.Context, _ int, msg *botapi.Message) (botapi.SendUpdateResponseObject, error) {
	chatID := msg.Chat.Id

	channelID, err := ids.NewChannelID("telegram", strconv.Itoa(chatID))
	if err != nil {
		h.log.ProcessMessageIssue(ctx, chatID, fmt.Errorf("making channel id: %w", err))

		return botapi.SendUpdate204Response{}, nil
	}

	messageID, err := ids.NewMessageID(channelID, strconv.Itoa(msg.MessageId))
	if err != nil {
		h.log.ProcessMessageIssue(ctx, chatID, fmt.Errorf("making message id: %w", err))

		return botapi.SendUpdate204Response{}, nil
	}

	userID, err := ids.NewUserID("telegram", strconv.Itoa(msg.From.Id))
	if err != nil {
		h.log.ProcessMessageIssue(ctx, chatID, fmt.Errorf("making user id: %w", err))

		return botapi.SendUpdate204Response{}, nil
	}

	var messageOptions []entities.NewMessageOption
	var text components.MessageText
	if msg.Text != nil && *msg.Text != "" {
		text, err = components.NewMessageText(*msg.Text)
		if err != nil {
			h.log.ProcessMessageIssue(ctx, chatID, fmt.Errorf("making message text: %w", err))

			return botapi.SendUpdate204Response{}, nil
		}
		messageOptions = append(messageOptions, entities.WithText(text))
	}

	message, err := entities.NewMessage(messageID, userID, messageOptions...)
	if err != nil {
		h.log.ProcessMessageIssue(ctx, chatID, fmt.Errorf("making message entity: %w", err))

		return botapi.SendUpdate204Response{}, nil
	}

	h.log.ProcessMessageStart(ctx, chatID, text.Text())
	startTime := time.Now()

	if err := h.srv.ReceiveNewMessageEvent(ctx, message); err != nil {
		h.log.ProcessMessageIssue(ctx, chatID, fmt.Errorf("processing new message: %w", err))
		return botapi.SendUpdate204Response{}, nil
	}

	duration := time.Since(startTime)
	h.log.ProcessMessageSuccess(ctx, chatID, duration.String())

	return botapi.SendUpdate204Response{}, nil
}

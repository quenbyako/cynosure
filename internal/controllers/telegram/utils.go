package telegram

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	botapi "github.com/quenbyako/cynosure/contrib/tg-openapi/gen/go/botapi"
	"go.opentelemetry.io/otel/trace"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

func (h *Handler) identifyUser(ctx context.Context, msg *botapi.Message) (ids.UserID, error) {
	var nickname, firstName, lastName string
	if msg.From.Username != nil {
		nickname = *msg.From.Username
	}

	firstName = msg.From.FirstName
	if msg.From.LastName != nil {
		lastName = *msg.From.LastName
	}

	userID, err := h.users.EnsureUser(ctx, strconv.Itoa(msg.From.Id), nickname, firstName, lastName)
	if err != nil {
		return ids.UserID{}, fmt.Errorf("looking up user by telegram id: %w", err)
	}

	return userID, nil
}

func (h *Handler) sendRateLimitedMessage(ctx context.Context, msg *botapi.Message) {
	traceID := trace.SpanFromContext(ctx).SpanContext().TraceID()

	text := "Sorry, I'm currently overwhelmed with requests. Please try again in a moment."
	if traceID.IsValid() {
		text += fmt.Sprintf(" (trace id: %s)", traceID.String())
	}

	_, err := h.client.SendMessageWithResponse(ctx, botapi.SendMessageJSONRequestBody{
		ChatId:                  msg.Chat.Id,
		Text:                    text,
		MessageThreadId:         msg.MessageThreadId,
		AllowPaidBroadcast:      nil,
		BusinessConnectionId:    nil,
		DirectMessagesTopicId:   nil,
		DisableNotification:     nil,
		Entities:                nil,
		LinkPreviewOptions:      nil,
		MessageEffectId:         nil,
		ParseMode:               nil,
		ProtectContent:          nil,
		ReplyMarkup:             nil,
		ReplyParameters:         nil,
		SuggestedPostParameters: nil,
	})
	if err != nil {
		h.log.ProcessMessageIssue(ctx, msg.Chat.Id,
			fmt.Errorf("sending rate limit message: %w", err),
		)
	}
}

func (h *Handler) upsertTelegramMessage(
	ctx context.Context, chatID, threadID, msgID int, text string,
) (sentMessageID int, err error) {
	if msgID > 0 {
		return h.updateTelegramMessage(ctx, chatID, threadID, msgID, text)
	}

	return h.createTelegramMessage(ctx, chatID, threadID, msgID, text)
}

func (h *Handler) updateTelegramMessage(
	ctx context.Context, chatID, _, msgID int, text string,
) (sentMessageID int, err error) {
	_, err = h.client.EditMessageTextWithResponse(ctx, botapi.EditMessageTextJSONRequestBody{
		ChatId:               &chatID,
		MessageId:            &msgID,
		Text:                 text,
		BusinessConnectionId: nil,
		Entities:             nil,
		InlineMessageId:      nil,
		LinkPreviewOptions:   nil,
		ParseMode:            nil,
		ReplyMarkup:          nil,
	})
	if err != nil {
		return 0, fmt.Errorf("edit message: %w", err)
	}

	return msgID, nil
}

func (h *Handler) createTelegramMessage(
	ctx context.Context, chatID, threadID, _ int, text string,
) (sentMessageID int, err error) {
	var thread *int
	if threadID > 0 {
		thread = &threadID
	}

	//nolint:exhaustruct // too many optional fields.
	resp, err := h.client.SendMessageWithResponse(ctx, botapi.SendMessageJSONRequestBody{
		ChatId:          chatID,
		Text:            text,
		MessageThreadId: thread,
	})
	if err != nil {
		return 0, fmt.Errorf("send message: %w", err)
	}

	sentMessageID = resp.JSON200.Result.MessageId
	if sentMessageID <= 0 {
		return 0, ErrInvalidMessageID
	}

	return sentMessageID, nil
}

func (h *Handler) formatThread(msg *botapi.Message) string {
	thread := strconv.Itoa(msg.Chat.Id)
	if msg.MessageThreadId != nil {
		thread += "_" + strconv.Itoa(*msg.MessageThreadId)
	}

	return thread
}

type noContentResponse struct{}

func (noContentResponse) VisitSendUpdateResponse(w http.ResponseWriter) error {
	w.WriteHeader(http.StatusNoContent)
	return nil
}

//nolint:containedctx // that's extension for context mechanism.
type merged struct {
	context.Context
	valuesOnly context.Context
}

func ctxMergeValuesOnly(ctx, values context.Context) context.Context {
	return &merged{Context: ctx, valuesOnly: context.WithoutCancel(values)}
}

//nolint:ireturn // context.Value returns any
func (d *merged) Value(k any) any {
	if val := d.valuesOnly.Value(k); val != nil {
		return val
	}

	return d.Context.Value(k)
}

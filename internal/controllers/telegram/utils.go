package telegram

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	botapi "github.com/quenbyako/cynosure/contrib/tg-openapi/gen/go/botapi"
	"go.opentelemetry.io/otel/trace"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/ratelimiter"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

func (h *Handler) identifyUser(ctx context.Context, msg *botapi.Message) (ids.UserID, error) {
	tgIDStr := strconv.Itoa(msg.From.Id)

	var nickname, firstName, lastName string
	if msg.From.Username != nil {
		nickname = *msg.From.Username
	}

	firstName = msg.From.FirstName
	if msg.From.LastName != nil {
		lastName = *msg.From.LastName
	}

	userID, err := h.users.EnsureUser(ctx, tgIDStr, nickname, firstName, lastName)
	if err != nil {
		return ids.UserID{}, fmt.Errorf("looking up user by telegram id: %w", err)
	}

	return userID, nil
}

func (h *Handler) sendRateLimitedMessage(
	ctx context.Context, chatID int, threadID *int, err error,
) {
	text := h.buildRateLimitedMessage(ctx, err)

	//nolint:exhaustruct // too many optional fields.
	params := botapi.SendMessageJSONRequestBody{
		ChatId:          chatID,
		Text:            text,
		MessageThreadId: threadID,
	}

	if _, err := h.client.SendMessageWithResponse(ctx, params); err != nil {
		h.log.ProcessMessageIssue(ctx, chatID,
			fmt.Errorf("sending rate limit message: %w", err),
		)
	}
}

func (h *Handler) buildRateLimitedMessage(ctx context.Context, err error) string {
	var (
		retryAfter   string
		rateLimitErr *ratelimiter.RateLimitExceededError
	)

	if errors.As(err, &rateLimitErr) {
		retryAfter = formatRetryAfter(rateLimitErr.RetryAt())
	}

	text := "Sorry, you've reached the message rate limit." + retryAfter +
		"\n\nYou can increase your limit, by getting /premium"

	traceID := trace.SpanFromContext(ctx).SpanContext().TraceID()
	if traceID.IsValid() {
		text += fmt.Sprintf(" (trace id: %s)", traceID.String())
	}

	return text
}

func (h *Handler) sendErrorMessage(ctx context.Context, chatID int, threadID *int) {
	//nolint:exhaustruct // too many optional fields.
	params := botapi.SendMessageJSONRequestBody{
		ChatId:          chatID,
		Text:            buildErrorText(ctx),
		ParseMode:       ptr("HTML"),
		MessageThreadId: threadID,
	}

	resp, err := h.client.SendMessageWithResponse(ctx, params)
	if err != nil {
		h.log.ProcessMessageIssue(ctx, chatID,
			fmt.Errorf("sending error message (network error): %w", err),
		)

		return
	}

	if resp.StatusCode() != http.StatusOK {
		h.log.ProcessMessageIssue(ctx, chatID,
			fmt.Errorf(
				"%w (api error %d): %s", ErrSendMessageFailed, resp.StatusCode(), string(resp.Body),
			),
		)
	}
}

func buildErrorText(ctx context.Context) string {
	traceID := trace.SpanFromContext(ctx).SpanContext().TraceID()
	text := "Oops! I've got some problems 😳\n\n"

	if traceID.IsValid() {
		text += fmt.Sprintf("Trace ID: <code>%s</code>\n", traceID.String())
	}

	return text + "Please, contact @quenbyako for help."
}

func (h *Handler) sendTooLargeMessage(ctx context.Context, chatID int, tgThreadID *int) {
	text := "Your message is too long, please shorten it and try again."

	//nolint:exhaustruct // too many optional fields.
	params := botapi.SendMessageJSONRequestBody{
		ChatId:          chatID,
		Text:            text,
		MessageThreadId: tgThreadID,
	}

	if _, err := h.client.SendMessageWithResponse(ctx, params); err != nil {
		h.log.ProcessMessageIssue(ctx, chatID,
			fmt.Errorf("sending too large message error: %w", err),
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
	//nolint:exhaustruct // too many optional fields.
	params := botapi.EditMessageTextJSONRequestBody{
		ChatId:    &chatID,
		MessageId: &msgID,
		Text:      text,
	}

	_, err = h.client.EditMessageTextWithResponse(ctx, params)
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
	params := botapi.SendMessageJSONRequestBody{
		ChatId:          chatID,
		Text:            text,
		MessageThreadId: thread,
	}

	resp, err := h.client.SendMessageWithResponse(ctx, params)
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

func formatRetryAfter(retryAt time.Time) string {
	retryIn := time.Until(retryAt).Round(time.Minute)
	if retryIn < time.Minute {
		retryIn = time.Until(retryAt).Round(time.Second)
	}

	if retryIn > 0 {
		return fmt.Sprintf(" Please try again in about %s.", retryIn.String())
	}

	return ""
}

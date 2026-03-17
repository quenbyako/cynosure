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
	"golang.org/x/time/rate"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/identitymanager"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

//nolint:ireturn // polymorphic bot api response
func (h *Handler) SendUpdate(
	ctx context.Context, request botapi.SendUpdateRequestObject,
) (botapi.SendUpdateResponseObject, error) {
	ctx, span := h.tracer.Start(ctx, "SendUpdate")
	defer span.End()

	update := request.Body
	if update == nil {
		return nil, errors.New("update is nil")
	}

	updateID := update.UpdateId
	switch {
	case update.Message != nil:
		h.processMessage(ctx, updateID, update.Message)

		return noContentResponse{}, nil
	default:
		// Unknown update type, ignore
		return noContentResponse{}, nil
	}
}

//nolint:ireturn // polymorphic response
func (h *Handler) processMessage(
	requestCtx context.Context, _ int, msg *botapi.Message,
) {
	chatID := msg.Chat.Id

	if msg.Chat.Type != "private" {
		// Only supporting private chats for now to avoid group spamming
		return
	}

	// We resolve basic info synchronously to ensure we can respond with error
	// if something is fundamentally wrong.
	// But the actual heavy generation happens in background.
	var nickname, firstName, lastName string
	if msg.From.Username != nil {
		nickname = *msg.From.Username
	}

	firstName = msg.From.FirstName
	if msg.From.LastName != nil {
		lastName = *msg.From.LastName
	}

	userID, err := h.users.EnsureUser(
		requestCtx, strconv.Itoa(msg.From.Id), nickname, firstName, lastName,
	)
	if err != nil {
		if errors.Is(err, identitymanager.ErrRateLimited) {
			traceID := trace.SpanFromContext(requestCtx).SpanContext().TraceID()

			text := "Sorry, I'm currently overwhelmed with requests. Please try again in a moment."
			if traceID.IsValid() {
				text += fmt.Sprintf(" (trace id: %s)", traceID.String())
			}

			_, sendMessageErr := h.client.SendMessageWithResponse(
				requestCtx, botapi.SendMessageJSONRequestBody{
					ChatId:                  chatID,
					MessageThreadId:         msg.MessageThreadId,
					Text:                    text,
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
				},
			)
			if sendMessageErr != nil {
				h.log.ProcessMessageIssue(
					requestCtx, chatID, fmt.Errorf("sending error message: %w", sendMessageErr),
				)
			}

			return
		}

		h.log.ProcessMessageIssue(requestCtx, chatID, fmt.Errorf("making user id: %w", err))

		return
	}

	thread := strconv.Itoa(msg.Chat.Id)
	if msg.MessageThreadId != nil {
		thread += "_" + strconv.Itoa(*msg.MessageThreadId)
	}

	threadID, err := ids.NewThreadID(userID, thread)
	if err != nil {
		h.log.ProcessMessageIssue(requestCtx, chatID, fmt.Errorf("making thread id: %w", err))
		return
	}

	var text string
	if msg.Text != nil && *msg.Text != "" {
		text = *msg.Text
	}

	if text == "" {
		return
	}

	userMessage, err := messages.NewMessageUser(text)
	if err != nil {
		h.log.ProcessMessageIssue(requestCtx, chatID, fmt.Errorf("making user message: %w", err))
		return
	}

	// Detach processing to avoid Telegram timeout (and subsequent retries)
	go func(ctx context.Context) {
		h.log.ProcessMessageStart(ctx, chatID, text)

		startTime := time.Now()

		response, err := h.srv.GenerateResponse(ctx, threadID, userMessage)
		if err != nil {
			h.log.ProcessMessageIssue(ctx, chatID, fmt.Errorf("processing new message: %w", err))
			return
		}

		var (
			tgMsgID      *int
			accumulated  string
			lastSentText string
			limiter      = rate.NewLimiter(rate.Every(h.updateInterval), 1)
		)

		for res, err := range response {
			if err != nil {
				h.log.ProcessMessageIssue(ctx, chatID, fmt.Errorf("streaming response: %w", err))
				break
			}

			switch res := res.(type) {
			case messages.MessageAssistant:
				accumulated = res.Content()
			case messages.MessageToolError:
				accumulated += "\n\nTool error: " + string(res.Content())
			case messages.MessageToolRequest:
				accumulated += "\n\nTool request: " + res.ToolName()
			case messages.MessageToolResponse:
				accumulated += "\n\nTool response: " + string(res.Content())
			case messages.MessageUser:
				// ignoring user messages
			default:
				h.log.ProcessMessageIssue(ctx, chatID,
					fmt.Errorf("unexpected messages.Message: %#v", res),
				)

				return
			}

			if accumulated == "" {
				continue
			}

			if tgMsgID == nil {
				// First update: send new message
				resp, err := h.client.SendMessageWithResponse(
					ctx, botapi.SendMessageJSONRequestBody{
						ChatId:                  chatID,
						MessageThreadId:         msg.MessageThreadId,
						Text:                    accumulated,
						BusinessConnectionId:    nil,
						Entities:                nil,
						LinkPreviewOptions:      nil,
						ParseMode:               nil,
						ReplyMarkup:             nil,
						AllowPaidBroadcast:      nil,
						DirectMessagesTopicId:   nil,
						DisableNotification:     nil,
						MessageEffectId:         nil,
						ProtectContent:          nil,
						ReplyParameters:         nil,
						SuggestedPostParameters: nil,
					},
				)
				if err != nil {
					h.log.ProcessMessageIssue(
						ctx, chatID, fmt.Errorf("send initial message: %w", err),
					)

					continue
				}

				if resp.JSON200 != nil && resp.JSON200.Result.MessageId != 0 {
					id := resp.JSON200.Result.MessageId
					tgMsgID = &id
					lastSentText = accumulated
				} else if resp.JSONDefault != nil {
					h.log.ProcessMessageIssue(
						ctx, chatID,
						fmt.Errorf("send initial message: %v", resp.JSONDefault.Description),
					)

					break
				}

				continue
			}

			if accumulated != lastSentText && limiter.Allow() {
				_, err := h.client.EditMessageTextWithResponse(
					ctx, botapi.EditMessageTextJSONRequestBody{
						ChatId:               &chatID,
						MessageId:            tgMsgID,
						Text:                 accumulated,
						BusinessConnectionId: nil,
						Entities:             nil,
						InlineMessageId:      nil,
						LinkPreviewOptions:   nil,
						ParseMode:            nil,
						ReplyMarkup:          nil,
					},
				)
				if err != nil {
					h.log.ProcessMessageIssue(ctx, chatID, fmt.Errorf("edit message: %w", err))
					continue
				}

				lastSentText = accumulated
			}
		}

		if tgMsgID != nil && accumulated != lastSentText {
			_, _ = h.client.EditMessageTextWithResponse(ctx, botapi.EditMessageTextJSONRequestBody{
				ChatId:               &chatID,
				MessageId:            tgMsgID,
				Text:                 accumulated,
				BusinessConnectionId: nil,
				Entities:             nil,
				InlineMessageId:      nil,
				LinkPreviewOptions:   nil,
				ParseMode:            nil,
				ReplyMarkup:          nil,
			})
		}

		duration := time.Since(startTime)
		h.log.ProcessMessageSuccess(ctx, chatID, duration.String())
	}(ctxMergeValuesOnly(h.lifecycleCtx, requestCtx))
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

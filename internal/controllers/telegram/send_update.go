package telegram

import (
	"context"
	"fmt"
	"strconv"
	"time"

	botapi "github.com/quenbyako/cynosure/contrib/tg-openapi/gen/go/botapi"
	"golang.org/x/time/rate"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

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

func (h *Handler) processMessage(requestCtx context.Context, _ int, msg *botapi.Message) (botapi.SendUpdateResponseObject, error) {
	chatID := msg.Chat.Id

	if msg.Chat.Type != "private" {
		// Only supporting private chats for now to avoid group spamming
		return botapi.SendUpdate204Response{}, nil
	}

	// We resolve basic info synchronously to ensure we can respond with error if something is fundamentally wrong.
	// But the actual heavy generation happens in background.
	userID, err := h.users.EnsureUser(requestCtx, "telegram", strconv.Itoa(msg.From.Id))
	if err != nil {
		h.log.ProcessMessageIssue(requestCtx, chatID, fmt.Errorf("making user id: %w", err))
		return botapi.SendUpdate204Response{}, nil
	}

	thread := strconv.Itoa(msg.Chat.Id)
	if msg.MessageThreadId != nil {
		thread += "_" + strconv.Itoa(*msg.MessageThreadId)
	}

	threadID, err := ids.NewThreadID(userID, thread)
	if err != nil {
		h.log.ProcessMessageIssue(requestCtx, chatID, fmt.Errorf("making thread id: %w", err))
		return botapi.SendUpdate204Response{}, nil
	}

	var text string
	if msg.Text != nil && *msg.Text != "" {
		text = *msg.Text
	}
	if text == "" {
		return botapi.SendUpdate204Response{}, nil
	}

	userMessage, err := messages.NewMessageUser(text)
	if err != nil {
		h.log.ProcessMessageIssue(requestCtx, chatID, fmt.Errorf("making user message: %w", err))
		return botapi.SendUpdate204Response{}, nil
	}

	// Detach processing to avoid Telegram timeout (and subsequent retries)
	go func() {
		// Use lifecycleCtx for background work to ensure it stops on SIGINT
		ctx := h.lifecycleCtx

		h.log.ProcessMessageStart(ctx, chatID, text)
		startTime := time.Now()

		response, err := h.srv.GenerateResponse(ctx, threadID, userMessage)
		if err != nil {
			h.log.ProcessMessageIssue(ctx, chatID, fmt.Errorf("processing new message: %w", err))
			return
		}

		var (
			tgMsgId      *int
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
				accumulated = res.Text()
			case messages.MessageToolError:
				accumulated += fmt.Sprintf("\n\nTool error: %s", string(res.Content()))
			case messages.MessageToolRequest:
				accumulated += fmt.Sprintf("\n\nTool request: %s", res.ToolName())
			case messages.MessageToolResponse:
				accumulated += fmt.Sprintf("\n\nTool response: %s", string(res.Content()))
			case messages.MessageUser:
				// ignoring user messages
			default:
				panic(fmt.Sprintf("unexpected messages.Message: %#v", res))
			}

			if accumulated == "" {
				continue
			}

			if tgMsgId == nil {
				// First update: send new message
				resp, err := h.client.SendMessageWithResponse(ctx, botapi.SendMessageJSONRequestBody{
					ChatId:          chatID,
					MessageThreadId: msg.MessageThreadId,
					Text:            accumulated,
				})
				if err != nil {
					h.log.ProcessMessageIssue(ctx, chatID, fmt.Errorf("send initial message: %w", err))
					continue
				}

				if resp.JSON200 != nil && resp.JSON200.Result.MessageId != 0 {
					id := resp.JSON200.Result.MessageId
					tgMsgId = &id
					lastSentText = accumulated
				} else if resp.JSONDefault != nil {
					h.log.ProcessMessageIssue(ctx, chatID, fmt.Errorf("send initial message: %v", resp.JSONDefault.Description))
					break
				}
				continue
			}

			if accumulated != lastSentText && limiter.Allow() {
				_, err := h.client.EditMessageTextWithResponse(ctx, botapi.EditMessageTextJSONRequestBody{
					ChatId:    &chatID,
					MessageId: tgMsgId,
					Text:      accumulated,
				})
				if err != nil {
					h.log.ProcessMessageIssue(ctx, chatID, fmt.Errorf("edit message: %v", err))
					continue
				}
				lastSentText = accumulated
			}
		}

		if tgMsgId != nil && accumulated != lastSentText {
			_, _ = h.client.EditMessageTextWithResponse(ctx, botapi.EditMessageTextJSONRequestBody{
				ChatId:    &chatID,
				MessageId: tgMsgId,
				Text:      accumulated,
			})
		}

		duration := time.Since(startTime)
		h.log.ProcessMessageSuccess(ctx, chatID, duration.String())
	}()

	return botapi.SendUpdate204Response{}, nil
}

package telegram

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"time"

	botapi "github.com/quenbyako/cynosure/contrib/tg-openapi/gen/go/botapi"
	"golang.org/x/time/rate"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/identitymanager"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

func (h *Handler) processMessage(requestCtx context.Context, msg *botapi.Message) {
	if msg.Chat.Type != "private" {
		return
	}

	userID, err := h.identifyUser(requestCtx, msg)
	if err != nil {
		h.handleUserIdentificationError(requestCtx, msg, err)
		return
	}

	threadID, err := ids.NewThreadID(userID, h.formatThread(msg))
	if err != nil {
		h.log.ProcessMessageIssue(requestCtx, msg.Chat.Id, fmt.Errorf("making thread id: %w", err))
		return
	}

	text := ""
	if msg.Text != nil {
		text = *msg.Text
	}

	if text == "" {
		return
	}

	h.dispatchProcessing(requestCtx, msg, threadID, text)
}

func (h *Handler) dispatchProcessing(
	ctx context.Context, msg *botapi.Message,
	threadID ids.ThreadID, text string,
) {
	userMessage, err := messages.NewMessageUser(text)
	if errors.Is(err, messages.ErrMessageTooLarge) {
		h.sendTooLargeMessage(ctx, msg.Chat.Id, msg.MessageThreadId)

		return
	} else if err != nil {
		h.log.ProcessMessageIssue(ctx, msg.Chat.Id,
			fmt.Errorf("making user message: %w", err),
		)

		return
	}

	var msgThreadID int
	if msg.MessageThreadId != nil && *msg.MessageThreadId > 0 {
		msgThreadID = *msg.MessageThreadId
	}

	ok := h.pool.Submit(ctx, asyncProcessRequest{
		userMessage: userMessage,
		threadID:    threadID,
		chatID:      msg.Chat.Id,
		tgThreadID:  msgThreadID,
	})
	if !ok {
		// TODO: add metrics to detect, how many messages were dropped due to non running pool.
		h.log.ProcessMessageIssue(ctx, msg.Chat.Id, fmt.Errorf("failed to submit async request, pool is not working"))
	}
}

func (h *Handler) handleUserIdentificationError(
	ctx context.Context, msg *botapi.Message, err error,
) {
	if errors.Is(err, identitymanager.ErrRateLimited) {
		h.sendRateLimitedMessage(ctx, msg)
		return
	}

	h.log.ProcessMessageIssue(ctx, msg.Chat.Id, fmt.Errorf("making user id: %w", err))
}

type asyncProcessRequest struct {
	userMessage messages.MessageUser
	threadID    ids.ThreadID
	chatID      int
	tgThreadID  int
}

func (h *Handler) asyncProcess(ctx context.Context, req asyncProcessRequest) {
	h.log.ProcessMessageStart(ctx, req.chatID, req.userMessage.Content())

	startTime := time.Now()

	response, err := h.srv.GenerateResponse(ctx, req.threadID, req.userMessage)
	if err != nil {
		h.log.ProcessMessageIssue(ctx, req.chatID, fmt.Errorf("processing new message: %w", err))

		return
	}

	h.streamToTelegram(ctx, req.chatID, req.tgThreadID, response)

	duration := time.Since(startTime)
	h.log.ProcessMessageSuccess(ctx, req.chatID, duration.String())
}

func (h *Handler) streamToTelegram(
	ctx context.Context, chatID, threadID int, response iter.Seq2[messages.Message, error],
) {
	limiter := rate.NewLimiter(rate.Every(h.updateInterval), 1)

	state := h.processStream(ctx, chatID, threadID, response, limiter)

	h.finalizeStreaming(ctx, chatID, threadID, state, limiter)
}

type streamState struct {
	accumulated  string
	lastSentText string
	tgMsgID      int
}

func (h *Handler) processStream(
	ctx context.Context, chatID, threadID int,
	response iter.Seq2[messages.Message, error],
	limiter *rate.Limiter,
) (state streamState) {
	for res, err := range response {
		if err != nil {
			h.log.ProcessMessageIssue(ctx, chatID, fmt.Errorf("streaming response: %w", err))
			break
		}

		var ok bool

		state, ok = h.handleStreamItem(
			ctx, chatID, threadID, res, state, limiter,
		)

		if !ok {
			break
		}
	}

	return state
}

func (h *Handler) handleStreamItem(
	ctx context.Context, chatID, threadID int,
	res messages.Message,
	state streamState,
	limiter *rate.Limiter,
) (streamState, bool) {
	next, ok := h.accumulateMessage(ctx, chatID, res)
	if !ok {
		return state, false
	}

	state.accumulated = next

	if state.accumulated == "" || state.accumulated == state.lastSentText {
		return state, true
	}

	var throttleOk bool

	state.tgMsgID, state.lastSentText, throttleOk = h.throttleUpdate(
		ctx, chatID, threadID, state.tgMsgID,
		state.accumulated, state.lastSentText, limiter,
	)

	return state, throttleOk
}

func (h *Handler) throttleUpdate(
	ctx context.Context,
	chatID, threadID, tgMsgID int,
	accumulated, lastSentText string,
	limiter *rate.Limiter,
) (newID int, nextSentText string, ok bool) {
	if !limiter.Allow() {
		return tgMsgID, lastSentText, true
	}

	newID, err := h.upsertTelegramMessage(ctx, chatID, threadID, tgMsgID, accumulated)
	if err != nil {
		h.log.ProcessMessageIssue(ctx, chatID, fmt.Errorf("upserting message: %w", err))
		return tgMsgID, lastSentText, false
	}

	return newID, accumulated, true
}

func (h *Handler) finalizeStreaming(
	ctx context.Context,
	chatID, threadID int,
	state streamState,
	limiter *rate.Limiter,
) {
	if state.tgMsgID <= 0 || state.accumulated == state.lastSentText {
		return
	}

	if err := limiter.Wait(ctx); err != nil {
		h.log.ProcessMessageIssue(ctx, chatID, fmt.Errorf("waiting for limiter: %w", err))
		// WARN: it's totally okay to omit error here, cause we must complete
		// sending message. error here means that we failed ONLY for waiting,
		// and nothing more.
	}

	if _, err := h.upsertTelegramMessage(
		ctx, chatID, threadID, state.tgMsgID, state.accumulated,
	); err != nil {
		h.log.ProcessMessageIssue(ctx, chatID, fmt.Errorf("upserting message: %w", err))
	}
}

func (h *Handler) accumulateMessage(
	ctx context.Context, chatID int, res messages.Message,
) (string, bool) {
	switch res := res.(type) {
	case messages.MessageAssistant:
		return res.Content(), true
	case messages.MessageToolError:
		return "\n\nTool error: " + string(res.Content()), true
	case messages.MessageToolRequest:
		return "\n\nTool request: " + res.ToolName(), true
	case messages.MessageToolResponse:
		return "\n\nTool response: " + string(res.Content()), true
	case messages.MessageUser:
		return "", true
	default:
		h.log.ProcessMessageIssue(ctx, chatID,
			ErrInternalValidation("unexpected messages.Message: %#v", res),
		)

		return "", false
	}
}

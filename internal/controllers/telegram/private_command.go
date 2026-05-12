package telegram

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"unicode/utf16"

	botapi "github.com/quenbyako/cynosure/contrib/tg-openapi/gen/go/botapi"

	_ "embed"
)

var (
	//go:embed l10n/start.md
	startText string

	//go:embed l10n/help.md
	helpText string

	//go:embed l10n/premium.md
	premiumText string
)

func (h *Handler) processCommand(ctx context.Context, msg *botapi.Message) {
	cmdStr, ok := h.parseCommand(ctx, msg)
	if !ok {
		return
	}

	// note: telegram commands can be like /start@bot_name, so we should handle that.
	switch {
	case cmdStr == "/start" || strings.HasPrefix(cmdStr, "/start@"):
		h.handleStart(ctx, msg)
	case cmdStr == "/help" || strings.HasPrefix(cmdStr, "/help@"):
		h.handleHelp(ctx, msg)
	case cmdStr == "/premium" || strings.HasPrefix(cmdStr, "/premium@"):
		h.handlePremium(ctx, msg)
	default:
		h.log.ProcessMessageIssue(ctx, msg.Chat.Id, fmt.Errorf("%w: %s", ErrUnknownCommand, cmdStr))
	}
}

func (h *Handler) parseCommand(ctx context.Context, msg *botapi.Message) (string, bool) {
	if msg.Chat.Type != "private" || msg.Entities == nil || len(*msg.Entities) == 0 {
		return "", false
	}

	idx := slices.IndexFunc(*msg.Entities, func(entity botapi.MessageEntity) bool {
		return entity.Type == "bot_command"
	})
	if idx < 0 {
		return "", false
	}

	commandEntity := (*msg.Entities)[idx]
	text := ""

	if msg.Text != nil {
		text = *msg.Text
	}

	cmdStr, ok := extractEntity(&commandEntity, text)
	if !ok {
		h.log.ProcessMessageIssue(ctx, msg.Chat.Id, ErrInvalidCommandEntity)

		return "", false
	}

	return cmdStr, true
}

func (h *Handler) handleStart(ctx context.Context, msg *botapi.Message) {
	h.sendCommandMessage(ctx, msg, startText)
}

func (h *Handler) handleHelp(ctx context.Context, msg *botapi.Message) {
	h.sendCommandMessage(ctx, msg, helpText)
}

func (h *Handler) handlePremium(ctx context.Context, msg *botapi.Message) {
	h.sendCommandMessage(ctx, msg, premiumText)
}

func (h *Handler) sendCommandMessage(ctx context.Context, msg *botapi.Message, text string) {
	userID, err := h.identifyUser(ctx, msg)
	if err != nil {
		h.handleUserIdentificationError(ctx, msg, err)
		return
	}

	if err = h.users.InitializeAccount(ctx, userID); err != nil {
		h.log.ProcessMessageIssue(ctx, msg.Chat.Id, fmt.Errorf("initializing account: %w", err))
		h.sendErrorMessage(ctx, msg.Chat.Id, msg.MessageThreadId)

		return
	}

	h.sendMessage(ctx, msg.Chat.Id, msg.MessageThreadId, text)
}

func (h *Handler) sendMessage(ctx context.Context, chatID int, threadID *int, text string) {
	//nolint:exhaustruct // too many optional fields.
	params := botapi.SendMessageJSONRequestBody{
		ChatId:          chatID,
		Text:            text,
		ParseMode:       ptr("MarkdownV2"),
		MessageThreadId: threadID,
	}

	resp, err := h.client.SendMessageWithResponse(ctx, params)
	if err != nil {
		h.log.ProcessMessageIssue(ctx, chatID,
			fmt.Errorf("%w (network): %w", ErrWelcomeMessageFailed, err),
		)

		return
	}

	if resp.StatusCode() != http.StatusOK {
		h.log.ProcessMessageIssue(ctx, chatID,
			fmt.Errorf(
				"%w (api error %d): %s",
				ErrWelcomeMessageFailed,
				resp.StatusCode(),
				string(resp.Body),
			),
		)
	}
}

// note: telegram uses utf16 for lengths and offsets, so convert manually.
func extractEntity(entity *botapi.MessageEntity, text string) (string, bool) {
	if entity == nil {
		return "", false
	}

	u16 := utf16.Encode([]rune(text))
	if entity.Offset < 0 || entity.Offset+entity.Length > len(u16) {
		return "", false
	}

	res := string(utf16.Decode(u16[entity.Offset : entity.Offset+entity.Length]))

	return res, true
}

func ptr[T any](v T) *T {
	return &v
}

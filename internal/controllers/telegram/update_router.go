package telegram

import (
	"context"
	"slices"

	botapi "github.com/quenbyako/cynosure/contrib/tg-openapi/gen/go/botapi"
)

// SendUpdate handles incoming Telegram updates.
//
//nolint:ireturn // polymorphic object
func (h *Handler) SendUpdate(
	ctx context.Context, request botapi.SendUpdateRequestObject,
) (botapi.SendUpdateResponseObject, error) {
	if !h.pool.Running() {
		h.log.TelegramPoolNotRunning(ctx)

		return nil, ErrInternalValidation("work pool not started")
	}

	ctx, span := h.tracer.Start(ctx, "SendUpdate")
	defer span.End()

	update := request.Body
	if update == nil {
		return noContentResponse{}, ErrInternalValidation("empty body")
	}

	switch {
	case update.Message != nil &&
		update.Message.Entities != nil &&
		slices.ContainsFunc(*update.Message.Entities, func(entity botapi.MessageEntity) bool {
			return entity.Type == "bot_command"
		}):
		h.processCommand(ctx, update.Message)

	case update.Message != nil:
		h.processMessage(ctx, update.Message)
	}

	return noContentResponse{}, nil
}

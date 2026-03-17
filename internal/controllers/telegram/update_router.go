package telegram

import (
	"context"

	botapi "github.com/quenbyako/cynosure/contrib/tg-openapi/gen/go/botapi"
)

// SendUpdate handles incoming Telegram updates.
//
//nolint:ireturn // polymorphic object
func (h *Handler) SendUpdate(
	ctx context.Context, request botapi.SendUpdateRequestObject,
) (botapi.SendUpdateResponseObject, error) {
	ctx, span := h.tracer.Start(ctx, "SendUpdate")
	defer span.End()

	update := request.Body
	if update == nil {
		return noContentResponse{}, ErrInternalValidation("empty body")
	}

	if update.Message != nil {
		h.processMessage(ctx, update.Message)
	}

	return noContentResponse{}, nil
}

package chat

import (
	"context"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/messages"
)

type LogCallbacks interface {
	MaxTurnsReached(ctx context.Context, threadID, userID string)
	ToolCalled(ctx context.Context, threadID, userID string, toolNames []messages.MessageToolRequest)
}

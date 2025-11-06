package chat

import (
	"context"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/messages"
)

type LogCallbacks interface {
	MaxTurnsReached(ctx context.Context, threadID, userID string)
	ToolCalled(ctx context.Context, threadID, userID string, toolNames []messages.MessageToolRequest)
}

type NoOpLogCallbacks struct{}

var _ LogCallbacks = NoOpLogCallbacks{}

func (n NoOpLogCallbacks) MaxTurnsReached(ctx context.Context, threadID string, userID string) {}
func (n NoOpLogCallbacks) ToolCalled(ctx context.Context, threadID string, userID string, toolNames []messages.MessageToolRequest) {
}

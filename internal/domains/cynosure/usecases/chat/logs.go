package chat

import (
	"context"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

type LogCallbacks interface {
	MaxTurnsReached(ctx context.Context, threadID string)
	ToolCalled(ctx context.Context, threadID string, toolNames []messages.MessageToolRequest)
}

type NoOpLogCallbacks struct{}

var _ LogCallbacks = NoOpLogCallbacks{}

func (n NoOpLogCallbacks) MaxTurnsReached(ctx context.Context, threadID string) {}
func (n NoOpLogCallbacks) ToolCalled(
	ctx context.Context,
	threadID string,
	toolNames []messages.MessageToolRequest,
) {
}

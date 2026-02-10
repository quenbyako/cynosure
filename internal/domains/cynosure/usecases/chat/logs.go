package chat

import (
	"context"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

type LogCallbacks interface {
	MaxTurnsReached(ctx context.Context, threadID ids.ThreadID)
	ToolCalled(ctx context.Context, threadID ids.ThreadID, toolNames []messages.MessageToolRequest)
}

type NoOpLogCallbacks struct{}

var _ LogCallbacks = NoOpLogCallbacks{}

func (n NoOpLogCallbacks) MaxTurnsReached(ctx context.Context, threadID ids.ThreadID) {}
func (n NoOpLogCallbacks) ToolCalled(ctx context.Context, threadID ids.ThreadID, toolNames []messages.MessageToolRequest) {
}

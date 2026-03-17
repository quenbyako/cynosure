package chat

import (
	"context"
	"fmt"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

// AcceptToolResult records the output of an external tool execution.
//
// Role in Agent Loop:
// After the model requests a tool (via [Chat.AcceptAssistantMessage]), the infrastructure
// executes it and passes the result back here.
//
// Protocol Validation:
// This method ensures that the result corresponds to a tool call that:
// 1. Was actually requested by the assistant.
// 2. Hasn't been answered yet.
// 3. Belongs to the current conversation turn (since last user message).
func (c *Chat) AcceptToolResult(ctx context.Context, message messages.MessageTool) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.activeCalls[message.ToolCallID()]; !ok {
		return errToolIDNotPending(message.ToolCallID())
	}

	if err := c.thread.AddMessage(message); err != nil {
		return fmt.Errorf("adding tool result: %w", err)
	}

	if err := c.storage.UpdateThread(ctx, c.thread); err != nil {
		c.thread.Reset()
		return fmt.Errorf("saving thread after tool result: %w", err)
	}

	delete(c.activeCalls, message.ToolCallID())

	c.thread.ClearEvents()

	return nil
}

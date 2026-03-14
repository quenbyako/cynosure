package chat

import (
	"context"
	"fmt"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

// AcceptUserMessage incorporates a user's input into the conversation.
//
// Role in Agent Loop: This method initiates a new "turn" in the conversation.
// When a user speaks, the semantic context of the conversation changes
// significantly. Therefore, this is the ONLY point in the cycle where we
// perform expensive RAG operations.
func (c *Chat) AcceptUserMessage(ctx context.Context, message messages.MessageUser) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.thread.AddMessage(message); err != nil {
		return fmt.Errorf("adding user message to thread: %w", err)
	}

	toolbox, err := c.buildToolbox(ctx)
	if err != nil {
		c.thread.Reset() // Rollback: remove the message we just added
		return fmt.Errorf("updating context after user message: %w", err)
	}

	c.toolbox = toolbox

	if err := c.storage.UpdateThread(ctx, c.thread); err != nil {
		c.thread.Reset() // Rollback: remove message (tools map stays in memory but is harmless/stale)
		return fmt.Errorf("saving thread after user message: %w", err)
	}

	c.thread.ClearEvents()

	return nil
}

package chat

import (
	"context"
	"fmt"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/messages"
)

func (c *Chat) AddMessage(ctx context.Context, message messages.Message) error {
	if !message.Valid() {
		return fmt.Errorf("invalid message %T: %w", message, message.Validate())
	}

	if err := c.thread.AddMessage(message); err != nil {
		return fmt.Errorf("adding message to thread: %w", err)
	}
	reversedTools, err := pullToolsAndAccounts(ctx, c.tools, c.accounts, c.thread)
	if err != nil {
		c.thread.Reset()
		return fmt.Errorf("retrieving relevant tools: %w", err)
	}

	if err := c.storage.SaveThread(ctx, c.thread); err != nil {
		c.thread.Reset()
		return fmt.Errorf("saving thread: %w", err)
	}

	c.thread.ClearEvents()
	c.reversedTools = reversedTools

	return nil
}

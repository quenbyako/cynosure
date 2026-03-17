package tools

import (
	"context"
	"fmt"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

func (t *Tools) DeleteTool(ctx context.Context, tool ids.ToolID) error {
	err := t.q.DeleteTool(ctx, tool.ID())
	if err != nil {
		return fmt.Errorf("delete tool: %w", err)
	}

	return nil
}

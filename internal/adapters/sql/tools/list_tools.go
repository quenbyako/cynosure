package tools

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

func (t *Tools) ListTools(ctx context.Context, account ids.AccountID) ([]*entities.Tool, error) {
	rows, err := t.q.ListToolsForAccounts(ctx, []uuid.UUID{account.ID()})
	if err != nil {
		return nil, fmt.Errorf("list tools: %w", err)
	}

	tools := make([]*entities.Tool, 0, len(rows))
	for i := range rows {
		tool, err := mapToolFromListRow(account, &rows[i])
		if err != nil {
			return nil, err
		}

		tools = append(tools, tool)
	}

	return tools, nil
}

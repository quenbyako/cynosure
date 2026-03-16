package tools

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	db "github.com/quenbyako/cynosure/contrib/db/gen/go"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

func (t *Tools) GetTool(
	ctx context.Context,
	account ids.AccountID,
	tool ids.ToolID,
) (*entities.Tool, error) {
	row, err := t.q.GetTool(ctx, db.GetToolParams{
		AccountID: account.ID(),
		ToolID:    tool.ID(),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ports.ErrNotFound
		}

		return nil, fmt.Errorf("query tool: %w", err)
	}

	return mapToolFromGetRow(account, &row)
}

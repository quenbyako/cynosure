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
	for _, row := range rows {
		// Construct tool from row
		// Row type is ListToolsForAccountsRow

		id, err := ids.NewToolID(account, row.ID, ids.WithSlug(row.Name))
		if err != nil {
			return nil, fmt.Errorf("invalid tool id: %w", err)
		}

		var embedding [embeddingSize]float32

		if row.Embedding != nil {
			vec := row.Embedding.Slice()

			if len(vec) == embeddingSize {
				copy(embedding[:], vec)
			}
		}

		tool, err := entities.NewTool(
			id,
			row.Name,
			row.Description,
			row.Input,
			row.Output,
			entities.WithEmbedding(embedding),
		)
		if err != nil {
			return nil, fmt.Errorf("map tool: %w", err)
		}

		tools = append(tools, tool)
	}

	return tools, nil
}

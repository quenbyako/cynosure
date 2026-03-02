package tools

import (
	"context"
	"encoding/json"
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

	return mapTool(account, row)
}

func mapTool(account ids.AccountID, row db.GetToolRow) (*entities.Tool, error) {
	id, err := ids.NewToolID(account, row.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid tool id: %w", err)
	}

	// Embedding conversion
	var embedding [embeddingSize]float32

	if row.Embedding != nil {
		vec := row.Embedding.Slice()

		if len(vec) == embeddingSize {
			copy(embedding[:], vec)
		}
	}

	// Unmarshal schemas
	// row.Input is []byte (json)
	// row.Output is []byte (json)

	tool, err := entities.NewTool(
		id,
		row.AccountName,
		row.Name,
		row.Description,
		json.RawMessage(row.Input),
		json.RawMessage(row.Output),
		entities.WithEmbedding(embedding),
	)
	if err != nil {
		return nil, fmt.Errorf("new tool: %w", err)
	}

	return tool, nil
}

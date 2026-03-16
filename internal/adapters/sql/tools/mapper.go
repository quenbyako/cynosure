package tools

import (
	"encoding/json"
	"fmt"

	"github.com/pgvector/pgvector-go"
	db "github.com/quenbyako/cynosure/contrib/db/gen/go"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

func mapToolFromSearchRow(
	accID ids.AccountID,
	row *db.SearchToolsByEmbeddingRow,
) (*entities.Tool, error) {
	toolID, err := ids.NewToolID(accID, row.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid tool id: %w", err)
	}

	embedding := mapEmbedding(row.Embedding)

	tool, err := entities.NewTool(
		toolID,
		row.AccountName,
		row.Name,
		row.Description,
		row.Input,
		row.Output,
		entities.WithEmbedding(embedding),
	)
	if err != nil {
		return nil, fmt.Errorf("map tool: %w", err)
	}

	return tool, nil
}

func mapToolFromListRow(
	account ids.AccountID,
	row *db.ListToolsForAccountsRow,
) (*entities.Tool, error) {
	id, err := ids.NewToolID(account, row.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid tool id: %w", err)
	}

	embedding := mapEmbedding(row.Embedding)

	tool, err := entities.NewTool(
		id,
		row.AccountName,
		row.Name,
		row.Description,
		row.Input,
		row.Output,
		entities.WithEmbedding(embedding),
	)
	if err != nil {
		return nil, fmt.Errorf("map tool: %w", err)
	}

	return tool, nil
}

func mapToolFromGetRow(account ids.AccountID, row *db.GetToolRow) (*entities.Tool, error) {
	id, err := ids.NewToolID(account, row.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid tool id: %w", err)
	}

	embedding := mapEmbedding(row.Embedding)

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

func mapEmbedding(vec *pgvector.Vector) [embeddingSize]float32 {
	var embedding [embeddingSize]float32

	if vec != nil {
		v := vec.Slice()
		if len(v) == embeddingSize {
			copy(embedding[:], v)
		}
	}

	return embedding
}

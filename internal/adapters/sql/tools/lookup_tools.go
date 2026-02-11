package tools

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/pgvector/pgvector-go"

	db "github.com/quenbyako/cynosure/contrib/db/gen/go"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

func (t *Tools) LookupTools(
	ctx context.Context,
	user ids.UserID,
	embedding [embeddingSize]float32,
	limit int,
) ([]*entities.Tool, error) {
	accs, err := t.q.ListAccountIDs(ctx, user.ID())
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}

	if len(accs) == 0 {
		return []*entities.Tool{}, nil
	}

	accountIDs := make([]uuid.UUID, len(accs))
	accountMap := make(map[uuid.UUID]ids.AccountID)

	for i, acc := range accs {
		accountIDs[i] = acc.ID

		serverID, err := ids.NewServerID(acc.ServerID)
		if err != nil {
			continue
		}

		accID, err := ids.NewAccountID(user, serverID, acc.ID, ids.WithSlug(acc.Name))
		if err != nil {
			continue
		}

		accountMap[acc.ID] = accID
	}

	embedVec := pgvector.NewVector(embedding[:])

	rows, err := t.q.SearchToolsByEmbedding(ctx, db.SearchToolsByEmbeddingParams{
		AccountIds:     accountIDs,
		QueryEmbedding: &embedVec,
		LimitCount:     int64(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("search tools: %w", err)
	}

	tools := make([]*entities.Tool, 0, len(rows))
	for _, row := range rows {
		accID, ok := accountMap[row.AccountID]
		if !ok {
			continue
		}

		toolID, err := ids.NewToolID(accID, row.ID, ids.WithSlug(row.Name))
		if err != nil {
			return nil, fmt.Errorf("invalid tool id: %w", err)
		}

		var toolEmbedding [embeddingSize]float32

		if row.Embedding != nil {
			vec := row.Embedding.Slice()

			if len(vec) == embeddingSize {
				copy(toolEmbedding[:], vec)
			}
		}

		tool, err := entities.NewTool(
			toolID,
			row.Name,
			row.Description,
			row.Input,
			row.Output,
			entities.WithEmbedding(toolEmbedding),
		)
		if err != nil {
			return nil, fmt.Errorf("map tool: %w", err)
		}

		tools = append(tools, tool)
	}

	return tools, nil
}

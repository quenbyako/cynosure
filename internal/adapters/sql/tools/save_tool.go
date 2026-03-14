package tools

import (
	"context"
	"fmt"

	"github.com/pgvector/pgvector-go"
	db "github.com/quenbyako/cynosure/contrib/db/gen/go"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
)

func (t *Tools) SaveTool(ctx context.Context, tool entities.ToolReadOnly) error {
	embedding := tool.Embedding()
	embedVec := pgvector.NewVector(embedding[:])

	toolID := tool.ID()

	err := t.q.InsertAccountTool(ctx, db.InsertAccountToolParams{
		ID:          toolID.ID(),
		AccountID:   toolID.Account().ID(),
		Name:        tool.Name(),
		Description: tool.Description(),
		Input:       tool.InputSchema(),
		Output:      tool.OutputSchema(),
		Embedding:   &embedVec,
	})
	if err != nil {
		return fmt.Errorf("upsert tool: %w", err)
	}

	return nil
}

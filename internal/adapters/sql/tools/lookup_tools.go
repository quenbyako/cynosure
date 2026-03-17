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

// LookupTools performs semantic search for relevant tools.
//
//nolint:gocritic // hugeParam is architectural decision in domains
func (t *Tools) LookupTools(
	ctx context.Context,
	user ids.UserID,
	embedding [embeddingSize]float32,
	limit int,
) (foundTools []*entities.Tool, err error) {
	accs, err := t.q.ListAccountIDs(ctx, user.ID())
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}

	if len(accs) == 0 {
		return []*entities.Tool{}, nil
	}

	accountIDs, accountMap := mapAccountsForSearch(user, accs)
	embedVec := pgvector.NewVector(embedding[:])

	rows, err := t.q.SearchToolsByEmbedding(ctx, db.SearchToolsByEmbeddingParams{
		AccountIds:     accountIDs,
		QueryEmbedding: &embedVec,
		LimitCount:     int64(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("search tools: %w", err)
	}

	return mapToolsFromSearchRows(accountMap, rows)
}

func mapToolsFromSearchRows(
	accMap map[uuid.UUID]ids.AccountID,
	rows []db.SearchToolsByEmbeddingRow,
) ([]*entities.Tool, error) {
	tools := make([]*entities.Tool, 0, len(rows))

	for i := range rows {
		row := &rows[i]

		accID, ok := accMap[row.AccountID]
		if !ok {
			continue
		}

		tool, err := mapToolFromSearchRow(accID, row)
		if err != nil {
			return nil, err
		}

		tools = append(tools, tool)
	}

	return tools, nil
}

func mapAccountsForSearch(
	user ids.UserID,
	accs []db.ListAccountIDsRow,
) (accountIDs []uuid.UUID, accMap map[uuid.UUID]ids.AccountID) {
	accountIDs = make([]uuid.UUID, len(accs))
	accMap = make(map[uuid.UUID]ids.AccountID)

	for i, acc := range accs {
		accountIDs[i] = acc.ID

		serverID, err := ids.NewServerID(acc.ServerID)
		if err != nil {
			continue
		}

		accID, err := ids.NewAccountID(user, serverID, acc.ID)
		if err != nil {
			continue
		}

		accMap[acc.ID] = accID
	}

	return accountIDs, accMap
}

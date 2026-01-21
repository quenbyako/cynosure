package sql

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/quenbyako/cynosure/contrib/db/gen/go"

	"github.com/quenbyako/cynosure/internal/adapters/sql/datatransfer"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/ids"
)

func (a *Adapter) ListAccounts(ctx context.Context, user ids.UserID) ([]ids.AccountID, error) {
	rows, err := a.q.ListAccountIDs(ctx, user.ID())
	if err != nil {
		return nil, fmt.Errorf("listing accounts: %w", err)
	}

	result := make([]ids.AccountID, 0, len(rows))
	for _, row := range rows {
		serverID, err := ids.NewServerID(row.Server)
		if err != nil {
			return nil, fmt.Errorf("invalid server id for %s: %w", row.ID, err)
		}

		id, err := ids.NewAccountID(user, serverID, row.ID)
		if err != nil {
			return nil, fmt.Errorf("constructing account id for %s: %w", row.ID, err)
		}
		result = append(result, id)
	}

	return result, nil
}

func (a *Adapter) GetAccount(ctx context.Context, account ids.AccountID) (*entities.Account, error) {
	row, err := a.q.GetAccount(ctx, account.ID())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("account not found: %w", err)
		}
		return nil, fmt.Errorf("getting account: %w", err)
	}

	return a.mapAccount(ctx, row)
}

func (a *Adapter) GetAccountsBatch(ctx context.Context, accounts []ids.AccountID) ([]*entities.Account, error) {
	if len(accounts) == 0 {
		return []*entities.Account{}, nil
	}

	uuids := make([]uuid.UUID, len(accounts))
	for i, acc := range accounts {
		uuids[i] = acc.ID()
	}

	rows, err := a.q.GetAccountsBatch(ctx, uuids)
	if err != nil {
		return nil, fmt.Errorf("batch getting accounts: %w", err)
	}

	// Fetch tools for all these accounts in one go
	toolRows, err := a.q.ListToolsForAccounts(ctx, uuids)
	if err != nil {
		return nil, fmt.Errorf("fetching tools for accounts batch: %w", err)
	}

	toolsMap := make(map[uuid.UUID][]db.AgentsMcpTool)
	for _, t := range toolRows {
		toolsMap[t.Account] = append(toolsMap[t.Account], t)
	}

	result := make([]*entities.Account, len(rows))
	for i, row := range rows {
		// Map account
		acc, err := datatransfer.AccountFromDBWithTools(row, toolsMap[row.ID])
		if err != nil {
			return nil, fmt.Errorf("mapping account %s: %w", row.ID, err)
		}
		result[i] = acc
	}

	return result, nil
}

func (a *Adapter) SaveAccount(ctx context.Context, info entities.AccountReadOnly) error {
	tx, err := a.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	q := a.q.WithTx(tx)

	// Determine token UUID (if present)
	var tokenUUID pgtype.UUID

	err = q.UpsertAccount(ctx, db.UpsertAccountParams{
		ID:          info.ID().ID(),
		UserID:      info.ID().User().ID(),   // Use .User() instead of .UserID()
		Server:      info.ID().Server().ID(), // Use .Server() instead of .ServerID()
		Name:        info.Name(),
		Description: info.Description(),
		Token:       tokenUUID,
	})
	if err != nil {
		return fmt.Errorf("upserting account: %w", err)
	}

	// Replace tools (Delete + Insert)
	if err := q.DeleteAccountTools(ctx, info.ID().ID()); err != nil {
		return fmt.Errorf("clearing account tools: %w", err)
	}

	for _, tool := range info.Tools() {
		toolID := uuid.New()

		err = q.InsertAccountTool(ctx, db.InsertAccountToolParams{
			ID:      toolID,
			Account: info.ID().ID(),
			Name:    tool.Name(),
			Input:   tool.ParamsSchema(),
			Output:  tool.ResponseSchema(),
		})
		if err != nil {
			return fmt.Errorf("inserting tool %s: %w", tool.Name(), err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

func (a *Adapter) DeleteAccount(ctx context.Context, account ids.AccountID) error {
	if err := a.q.SoftDeleteAccount(ctx, account.ID()); err != nil {
		return fmt.Errorf("soft deleting account: %w", err)
	}

	return nil
}

// mapAccount fetches tools and maps the account row to a domain entity.
// This is the adapter-level query orchestration.
func (a *Adapter) mapAccount(ctx context.Context, row db.AgentsMcpAccount) (*entities.Account, error) {
	toolRows, err := a.q.ListToolsForAccounts(ctx, []uuid.UUID{row.ID})
	if err != nil {
		return nil, fmt.Errorf("fetching tools for account: %w", err)
	}
	return datatransfer.AccountFromDBWithTools(row, toolRows)
}

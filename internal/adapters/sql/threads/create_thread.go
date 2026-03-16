package threads

import (
	"context"
	"fmt"

	db "github.com/quenbyako/cynosure/contrib/db/gen/go"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

func (t *Threads) CreateThread(ctx context.Context, thread entities.ThreadReadOnly) error {
	id := thread.ID()

	transaction, err := t.tx.BeginTx(ctx, emptyTxOptions)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	//nolint:errcheck // it makes no sense to check the error in defer
	defer transaction.Rollback(ctx)

	qtx := t.q.WithTx(transaction)

	err = qtx.CreateThread(ctx, db.CreateThreadParams{
		ID:     id.String(),
		UserID: id.User().ID(),
	})
	if err != nil {
		return fmt.Errorf("create thread: %w", err)
	}

	if err := t.insertMessages(ctx, qtx, id.String(), thread.Messages()); err != nil {
		return err
	}

	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (t *Threads) insertMessages(
	ctx context.Context, qtx *db.Queries, threadID string, msgs []messages.Message,
) error {
	for i, msg := range msgs {
		pos := int64(i + 1)
		if err := t.insertMessage(ctx, qtx, threadID, pos, msg); err != nil {
			return fmt.Errorf("insert message %d: %w", i, err)
		}
	}

	return nil
}

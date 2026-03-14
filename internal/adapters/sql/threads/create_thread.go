package threads

import (
	"context"
	"fmt"

	db "github.com/quenbyako/cynosure/contrib/db/gen/go"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
)

func (t *Threads) CreateThread(ctx context.Context, thread entities.ThreadReadOnly) error {
	id := thread.ID()

	tx, err := t.tx.BeginTx(ctx, emptyTxOptions)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := t.q.WithTx(tx)

	err = qtx.CreateThread(ctx, db.CreateThreadParams{
		ID:     id.String(),
		UserID: id.User().ID(),
	})
	if err != nil {
		return fmt.Errorf("create thread: %w", err)
	}

	for i, msg := range thread.Messages() {
		pos := int64(i + 1)

		err := t.insertMessage(ctx, qtx, id.String(), pos, msg)
		if err != nil {
			return fmt.Errorf("insert message %d: %w", i, err)
		}
	}

	return tx.Commit(ctx)
}

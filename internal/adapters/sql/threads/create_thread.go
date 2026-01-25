package threads

import (
	"context"
	"fmt"

	db "github.com/quenbyako/cynosure/contrib/db/gen/go"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
)

func (t *Threads) CreateThread(ctx context.Context, thread entities.ThreadReadOnly) error {
	id := thread.ID()

	err := t.q.CreateThread(ctx, db.CreateThreadParams{
		ID:     id.String(),
		UserID: id.User().ID(),
	})
	if err != nil {
		return fmt.Errorf("create thread: %w", err)
	}

	return nil
}

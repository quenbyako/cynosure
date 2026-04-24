package threads

import (
	"context"
	"fmt"

	"github.com/quenbyako/cynosure/internal/adapters/sql/datatransfer"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

func (t *Threads) GetThread(ctx context.Context, id ids.ThreadID) (*entities.Thread, error) {
	rows, err := t.q.GetThreadWithMessages(ctx, id.String())
	if err != nil {
		return nil, fmt.Errorf("query thread: %w", err)
	}

	if len(rows) == 0 {
		return nil, ports.ErrNotFound
	}

	thread, err := datatransfer.ThreadFromRows(rows)
	if err != nil {
		return nil, fmt.Errorf("map thread: %w", err)
	}

	if len(thread.Messages(0)) == 0 {
		return nil, ports.ErrNotFound
	}

	return thread, nil
}

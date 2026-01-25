package servers

import (
	"context"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

func (s *Servers) DeleteServer(ctx context.Context, id ids.ServerID) error {
	// TODO(A2): implement DeleteServer for SQL storage
	// For now, return nil (idempotent - no error if server doesn't exist)
	return nil
}

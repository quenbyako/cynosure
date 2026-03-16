package servers

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/quenbyako/cynosure/internal/adapters/sql/datatransfer"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

func (s *Servers) GetServerInfo(
	ctx context.Context, id ids.ServerID,
) (*entities.ServerConfig, error) {
	row, err := s.q.GetServerInfo(ctx, id.ID())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ports.ErrNotFound
		}

		return nil, fmt.Errorf("failed to get server info: %w", err)
	}

	info, err := datatransfer.ServerInfoFromDB(row)
	if err != nil {
		return nil, fmt.Errorf("failed to convert server info: %w", err)
	}

	return info, nil
}

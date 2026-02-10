package servers

import (
	"context"
	"fmt"

	"github.com/quenbyako/cynosure/internal/adapters/sql/datatransfer"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
)

func (s *Servers) ListServers(ctx context.Context) ([]*entities.ServerConfig, error) {
	rows, err := s.q.ListServers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list servers: %w", err)
	}

	servers := make([]*entities.ServerConfig, 0, len(rows))
	for _, row := range rows {
		server, err := datatransfer.ServerInfoListFromDB(row)
		if err != nil {
			return nil, fmt.Errorf("failed to convert server info: %w", err)
		}

		servers = append(servers, server)
	}

	return servers, nil
}

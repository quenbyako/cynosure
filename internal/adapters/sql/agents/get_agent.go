package agents

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

func (s *Agents) GetAgent(ctx context.Context, id ids.AgentID) (*entities.Agent, error) {
	row, err := s.q.GetAgentSettings(ctx, id.ID())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ports.ErrNotFound
		}
		return nil, fmt.Errorf("query agent: %w", err)
	}

	agent, err := datatransfer.ToDomainAgent(row)
	if err != nil {
		return nil, fmt.Errorf("map agent: %w", err)
	}

	return agent, nil
}

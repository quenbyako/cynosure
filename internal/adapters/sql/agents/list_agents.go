package agents

import (
	"context"
	"fmt"

	"github.com/quenbyako/cynosure/internal/adapters/sql/datatransfer"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

func (s *Agents) ListAgents(ctx context.Context, user ids.UserID) ([]*entities.Agent, error) {
	// TODO: Filter by user_id when added to schema
	rows, err := s.q.ListAgentSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}

	agents := make([]*entities.Agent, len(rows))
	for i, row := range rows {
		agent, err := datatransfer.ToDomainAgent(row)
		if err != nil {
			return nil, fmt.Errorf("map agent: %w", err)
		}

		agents[i] = agent
	}

	return agents, nil
}

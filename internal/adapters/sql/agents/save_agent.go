package agents

import (
	"context"
	"fmt"

	"github.com/quenbyako/cynosure/internal/adapters/sql/datatransfer"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
)

func (s *Agents) SaveAgent(ctx context.Context, agent entities.AgentReadOnly) error {
	params, err := datatransfer.ToDBAgentParams(agent)
	if err != nil {
		return fmt.Errorf("map agent: %w", err)
	}

	err = s.q.UpsertAgentSettings(ctx, params)
	if err != nil {
		return fmt.Errorf("upsert agent: %w", err)
	}

	return nil
}

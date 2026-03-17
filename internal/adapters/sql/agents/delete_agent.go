package agents

import (
	"context"
	"fmt"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

func (s *Agents) DeleteAgent(ctx context.Context, id ids.AgentID) error {
	err := s.q.DeleteAgentSettings(ctx, id.ID())
	if err != nil {
		return fmt.Errorf("delete agent: %w", err)
	}

	return nil
}

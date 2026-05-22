package mcp

import (
	"context"
	"fmt"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

const (
	disableAgentName = "disable_agent"
	disableAgentDesc = "Deactivates an agent."
)

type (
	DisableAgentInput struct {
		AgentID string `json:"agent_id" jsonschema:"ID of the agent"`
	}
)

func (c *Controller) DisableAgent(
	ctx context.Context,
	input DisableAgentInput,
) (
	struct{},
	error,
) {
	userID, ok := FromContext(ctx)
	if !ok {
		return struct{}{}, ErrUnauthorized
	}

	agentID, err := ids.NewAgentIDFromString(userID, input.AgentID)
	if err != nil {
		return struct{}{}, fmt.Errorf("parsing agent ID: %w", err)
	}

	if err := c.accounts.DeleteAgent(ctx, userID, agentID); err != nil {
		return struct{}{}, fmt.Errorf("disabling agent: %w", err)
	}

	return struct{}{}, nil
}

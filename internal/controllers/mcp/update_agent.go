package mcp

import (
	"context"
	"fmt"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

const (
	updateAgentName = "update_agent"
	updateAgentDesc = "Updates parameters of an existing autonomous agent."
)

type (
	UpdateAgentInput struct {
		AgentID      string `json:"agent_id"                jsonschema:"ID of the agent to update"`
		Name         string `json:"name,omitempty"          jsonschema:"New name"`
		SystemPrompt string `json:"system_prompt,omitempty" jsonschema:"New system prompt"`
		ModelName    string `json:"model_name,omitempty"    jsonschema:"New model name"`
	}
)

func (c *Controller) UpdateAgent(
	ctx context.Context,
	input UpdateAgentInput,
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

	err = c.accounts.UpdateAgent(
		ctx, userID, agentID, input.ModelName, input.SystemPrompt,
	)
	if err != nil {
		return struct{}{}, fmt.Errorf("updating agent: %w", err)
	}

	return struct{}{}, nil
}

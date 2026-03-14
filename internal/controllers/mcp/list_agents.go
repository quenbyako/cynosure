package mcp

import (
	"context"
)

const (
	listAgentsName = "list_agents"
	listAgentsDesc = "List all agents belonging to the current user."
)

type (
	ListAgentsOutput struct {
		Agents []struct {
			AgentID   string `json:"agent_id"`
			Name      string `json:"name"`
			ModelName string `json:"model_name"`
		} `json:"agents"`
	}
)

func (c *Controller) ListAgents(ctx context.Context, _ struct{}) (ListAgentsOutput, error) {
	userID, ok := FromContext(ctx)
	if !ok {
		return ListAgentsOutput{}, ErrUnauthorized
	}

	_ = userID

	return ListAgentsOutput{}, ErrUnimplemented
}

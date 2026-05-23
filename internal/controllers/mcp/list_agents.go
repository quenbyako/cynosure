package mcp

import (
	"context"
	"fmt"
)

const (
	listAgentsName = "list_agents"
	listAgentsDesc = "List all agents belonging to the current user."
)

type (
	ListAgentsOutput struct {
		Agents []AgentItem `json:"agents"`
	}

	AgentItem struct {
		AgentID   string `json:"agent_id"`
		Name      string `json:"name"`
		ModelName string `json:"model_name"`
	}
)

func (c *Controller) ListAgents(ctx context.Context, _ struct{}) (ListAgentsOutput, error) {
	userID, ok := FromContext(ctx)
	if !ok {
		return ListAgentsOutput{}, ErrUnauthorized
	}

	agents, err := c.accounts.ListAgents(ctx, userID)
	if err != nil {
		return ListAgentsOutput{}, fmt.Errorf("listing agents: %w", err)
	}

	res := make([]AgentItem, len(agents))
	for i, a := range agents {
		res[i].AgentID = a.ID().ID().String()
		// res[i].Name = a.Name()
		res[i].ModelName = a.Model()
	}

	return ListAgentsOutput{Agents: res}, nil
}

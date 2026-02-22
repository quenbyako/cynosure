package mcp

import "context"

type (
	ListAgentsOutput struct {
		Agents []struct {
			AgentID   string `json:"agent_id"`
			Name      string `json:"name"`
			ModelName string `json:"model_name"`
		} `json:"agents"`
	}
)

func (c *Controller) ListAgents(_ context.Context, _ struct{}) (ListAgentsOutput, error) {
	userID := userID // TODO: get it from context
	_ = userID

	return ListAgentsOutput{}, nil
}

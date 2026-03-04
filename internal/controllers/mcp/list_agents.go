package mcp

import (
	"context"
	"errors"
	"fmt"
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
		return ListAgentsOutput{}, fmt.Errorf("missing user ID in context")
	}
	_ = userID

	return ListAgentsOutput{}, errors.New("unimplemented")
}

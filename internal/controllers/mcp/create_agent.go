package mcp

import (
	"context"
	"errors"
	"fmt"
)

type (
	CreateAgentInput struct {
		Name         string `json:"name" jsonschema:"Name of the agent"`
		SystemPrompt string `json:"system_prompt" jsonschema:"System prompt for instructions"`
		ModelName    string `json:"model_name" jsonschema:"Codename of the model to use"`
	}
	CreateAgentOutput struct {
		AgentID string `json:"agent_id" jsonschema:"ID of the created agent"`
	}
)

func (c *Controller) CreateAgent(ctx context.Context, in CreateAgentInput) (CreateAgentOutput, error) {
	userID, ok := FromContext(ctx)
	if !ok {
		return CreateAgentOutput{}, fmt.Errorf("missing user ID in context")
	}
	_ = userID

	return CreateAgentOutput{}, errors.New("unimplemented")
}

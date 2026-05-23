package mcp

import (
	"context"
	"fmt"
)

const (
	createAgentName = "create_agent"
	createAgentDesc = "Creates a new autonomous agent with specified prompt and model."
)

type (
	CreateAgentInput struct {
		Name         string `json:"name"          jsonschema:"Name of the agent"`
		SystemPrompt string `json:"system_prompt" jsonschema:"System prompt for instructions"`
		ModelName    string `json:"model_name"    jsonschema:"Codename of the model to use"`
	}
	CreateAgentOutput struct {
		AgentID string `json:"agent_id" jsonschema:"ID of the created agent"`
	}
)

func (c *Controller) CreateAgent(
	ctx context.Context,
	input CreateAgentInput,
) (
	CreateAgentOutput,
	error,
) {
	userID, ok := FromContext(ctx)
	if !ok {
		return CreateAgentOutput{}, ErrUnauthorized
	}

	agentID, err := c.accounts.CreateAgent(ctx, userID, input.ModelName, input.SystemPrompt)
	if err != nil {
		return CreateAgentOutput{}, fmt.Errorf("creating agent: %w", err)
	}

	return CreateAgentOutput{
		AgentID: agentID.ID().String(),
	}, nil
}

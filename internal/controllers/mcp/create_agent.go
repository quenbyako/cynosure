package mcp

import "context"

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

func (c *Controller) CreateAgent(_ context.Context, in CreateAgentInput) (CreateAgentOutput, error) {
	userID := userID // TODO: get it from context
	_ = userID

	return CreateAgentOutput{}, nil
}

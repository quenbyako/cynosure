package mcp

import "context"

type (
	UpdateAgentInput struct {
		AgentID      string `json:"agent_id" jsonschema:"ID of the agent to update"`
		Name         string `json:"name,omitempty" jsonschema:"New name"`
		SystemPrompt string `json:"system_prompt,omitempty" jsonschema:"New system prompt"`
		ModelName    string `json:"model_name,omitempty" jsonschema:"New model name"`
	}
)

func (c *Controller) UpdateAgent(_ context.Context, in UpdateAgentInput) (struct{}, error) {
	userID := userID // TODO: get it from context
	_ = userID

	return struct{}{}, nil
}

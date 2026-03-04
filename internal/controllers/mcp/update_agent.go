package mcp

import (
	"context"
	"errors"
	"fmt"
)

type (
	UpdateAgentInput struct {
		AgentID      string `json:"agent_id" jsonschema:"ID of the agent to update"`
		Name         string `json:"name,omitempty" jsonschema:"New name"`
		SystemPrompt string `json:"system_prompt,omitempty" jsonschema:"New system prompt"`
		ModelName    string `json:"model_name,omitempty" jsonschema:"New model name"`
	}
)

func (c *Controller) UpdateAgent(ctx context.Context, in UpdateAgentInput) (struct{}, error) {
	userID, ok := FromContext(ctx)
	if !ok {
		return struct{}{}, fmt.Errorf("missing user ID in context")
	}
	_ = userID

	return struct{}{}, errors.New("unimplemented")
}

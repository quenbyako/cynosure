package mcp

import (
	"context"
)

const (
	disableAgentName = "disable_agent"
	disableAgentDesc = "Deactivates an agent."
)

type (
	DisableAgentInput struct {
		AgentID string `json:"agent_id" jsonschema:"ID of the agent"`
	}
)

func (c *Controller) DisableAgent(ctx context.Context, in DisableAgentInput) (struct{}, error) {
	userID, ok := FromContext(ctx)
	if !ok {
		return struct{}{}, ErrUnauthorized
	}

	_ = userID

	return struct{}{}, ErrUnimplemented
}

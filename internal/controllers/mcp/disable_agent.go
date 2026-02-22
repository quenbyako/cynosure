package mcp

import "context"

type (
	DisableAgentInput struct {
		AgentID string `json:"agent_id" jsonschema:"ID of the agent"`
	}
)

func (c *Controller) DisableAgent(_ context.Context, in DisableAgentInput) (struct{}, error) {
	userID := userID // TODO: get it from context
	_ = userID

	return struct{}{}, nil
}

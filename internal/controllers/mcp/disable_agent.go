package mcp

import (
	"context"
	"errors"
	"fmt"
)

type (
	DisableAgentInput struct {
		AgentID string `json:"agent_id" jsonschema:"ID of the agent"`
	}
)

func (c *Controller) DisableAgent(ctx context.Context, in DisableAgentInput) (struct{}, error) {
	userID, ok := FromContext(ctx)
	if !ok {
		return struct{}{}, fmt.Errorf("missing user ID in context")
	}
	_ = userID

	return struct{}{}, errors.New("unimplemented")
}

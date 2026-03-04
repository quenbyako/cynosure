package mcp

import (
	"context"
	"errors"
	"fmt"
)

type (
	ReactivateMcpAccountInput struct {
		AccountID string `json:"account_id" jsonschema:"ID of the MCP account"`
	}
	ReactivateMcpAccountOutput struct {
		AuthURL string `json:"auth_url,omitempty" jsonschema:"Optional auth link if token expired"`
	}
)

func (c *Controller) ReactivateMcpAccount(ctx context.Context, in ReactivateMcpAccountInput) (ReactivateMcpAccountOutput, error) {
	userID, ok := FromContext(ctx)
	if !ok {
		return ReactivateMcpAccountOutput{}, fmt.Errorf("missing user ID in context")
	}
	_ = userID

	return ReactivateMcpAccountOutput{}, errors.New("unimplemented")
}

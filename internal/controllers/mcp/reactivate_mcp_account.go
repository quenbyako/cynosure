package mcp

import (
	"context"
)

const (
	reactivateMcpAccountName = "reactivate_mcp_account"
	reactivateMcpAccountDesc = "Reactivates a previously disabled MCP account."
)

type (
	ReactivateMcpAccountInput struct {
		AccountID string `json:"account_id" jsonschema:"ID of the MCP account"`
	}
	ReactivateMcpAccountOutput struct {
		AuthURL string `json:"auth_url,omitempty" jsonschema:"Optional auth link if token expired"`
	}
)

func (c *Controller) ReactivateMcpAccount(
	ctx context.Context,
	in ReactivateMcpAccountInput,
) (
	ReactivateMcpAccountOutput,
	error,
) {
	userID, ok := FromContext(ctx)
	if !ok {
		return ReactivateMcpAccountOutput{}, ErrUnauthorized
	}

	_ = userID

	return ReactivateMcpAccountOutput{}, ErrUnimplemented
}

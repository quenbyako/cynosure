package mcp

import (
	"context"
)

const (
	disableMcpAccountName = "disable_mcp_account"
	disableMcpAccountDesc = "Deactivates an MCP account, preventing tools from being used."
)

type (
	DisableMcpAccountInput struct {
		AccountID string `json:"account_id" jsonschema:"ID of the MCP account"`
	}
)

func (c *Controller) DisableMcpAccount(
	ctx context.Context,
	in DisableMcpAccountInput,
) (
	struct{},
	error,
) {
	userID, ok := FromContext(ctx)
	if !ok {
		return struct{}{}, ErrUnauthorized
	}

	_ = userID

	return struct{}{}, ErrUnimplemented
}

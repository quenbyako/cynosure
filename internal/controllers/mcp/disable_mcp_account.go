package mcp

import (
	"context"
	"fmt"
)

const (
	disableMcpAccountName = "disable_mcp_account"
	disableMcpAccountDesc = "Deactivates an MCP account, preventing tools from being used."
)

type (
	DisableMcpAccountInput struct {
		Name string `json:"name" jsonschema:"Name of the account to disable"`
	}
)

func (c *Controller) DisableMcpAccount(
	ctx context.Context,
	input DisableMcpAccountInput,
) (
	struct{},
	error,
) {
	userID, ok := FromContext(ctx)
	if !ok {
		return struct{}{}, ErrUnauthorized
	}

	if err := c.accounts.DisableAccountByName(ctx, userID, input.Name); err != nil {
		return struct{}{}, fmt.Errorf("disabling mcp account: %w", err)
	}

	return struct{}{}, nil
}

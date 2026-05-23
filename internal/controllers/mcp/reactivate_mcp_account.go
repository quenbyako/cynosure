package mcp

import (
	"context"
	"fmt"
)

const (
	reactivateMcpAccountName = "reactivate_mcp_account"
	reactivateMcpAccountDesc = "Reactivates a previously disabled MCP account."
)

type (
	ReactivateMcpAccountInput struct {
		Name string `json:"name" jsonschema:"Name of the account to reactivate (e.g. 'github')"`
	}
	ReactivateMcpAccountOutput struct {
		AuthURL string `json:"auth_url,omitempty" jsonschema:"Optional auth link if token expired"`
	}
)

func (c *Controller) ReactivateMcpAccount(
	ctx context.Context,
	input ReactivateMcpAccountInput,
) (
	ReactivateMcpAccountOutput,
	error,
) {
	userID, ok := FromContext(ctx)
	if !ok {
		return ReactivateMcpAccountOutput{}, ErrUnauthorized
	}

	if err := c.accounts.ReactivateAccountByName(ctx, userID, input.Name); err != nil {
		return ReactivateMcpAccountOutput{}, fmt.Errorf("reactivating account: %w", err)
	}

	return ReactivateMcpAccountOutput{}, nil
}

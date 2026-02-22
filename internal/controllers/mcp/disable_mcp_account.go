package mcp

import "context"

type (
	DisableMcpAccountInput struct {
		AccountID string `json:"account_id" jsonschema:"ID of the MCP account"`
	}
)

func (c *Controller) DisableMcpAccount(_ context.Context, in DisableMcpAccountInput) (struct{}, error) {
	userID := userID // TODO: get it from context
	_ = userID

	return struct{}{}, nil
}

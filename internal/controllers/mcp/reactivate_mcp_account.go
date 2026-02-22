package mcp

import "context"

type (
	ReactivateMcpAccountInput struct {
		AccountID string `json:"account_id" jsonschema:"ID of the MCP account"`
	}
	ReactivateMcpAccountOutput struct {
		AuthURL string `json:"auth_url,omitempty" jsonschema:"Optional auth link if token expired"`
	}
)

func (c *Controller) ReactivateMcpAccount(_ context.Context, in ReactivateMcpAccountInput) (ReactivateMcpAccountOutput, error) {
	userID := userID // TODO: get it from context
	_ = userID

	return ReactivateMcpAccountOutput{}, nil
}

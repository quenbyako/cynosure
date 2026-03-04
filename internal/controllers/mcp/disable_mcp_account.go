package mcp

import (
	"context"
	"errors"
	"fmt"
)

type (
	DisableMcpAccountInput struct {
		AccountID string `json:"account_id" jsonschema:"ID of the MCP account"`
	}
)

func (c *Controller) DisableMcpAccount(ctx context.Context, in DisableMcpAccountInput) (struct{}, error) {
	userID, ok := FromContext(ctx)
	if !ok {
		return struct{}{}, fmt.Errorf("missing user ID in context")
	}
	_ = userID

	return struct{}{}, errors.New("unimplemented")
}

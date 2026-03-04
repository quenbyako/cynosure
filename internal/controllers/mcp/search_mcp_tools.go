package mcp

import (
	"context"
	"fmt"
)

type (
	SearchMcpToolsInput struct {
		Query string `json:"query" jsonschema:"Search query text"`
		Limit int    `json:"limit,omitempty" jsonschema:"Max results"`
	}

	SearchMcpToolsOutput struct {
		Tools []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"tools"`
	}
)

func (c *Controller) SearchMcpTools(ctx context.Context, in SearchMcpToolsInput) (SearchMcpToolsOutput, error) {
	userID, ok := FromContext(ctx)
	if !ok {
		return SearchMcpToolsOutput{}, fmt.Errorf("missing user ID in context")
	}
	_ = userID

	return SearchMcpToolsOutput{}, nil
}

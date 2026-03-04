package mcp

import (
	"context"
	"errors"
	"fmt"
)

type (
	SearchMcpServersInput struct {
		Query string `json:"query" jsonschema:"Search query text"`
		Limit int    `json:"limit,omitempty" jsonschema:"Maximum number of results (default: 10)"`
	}
	SearchMcpServersOutput struct {
		Servers []struct {
			URL  string `json:"url"`
			Name string `json:"name"`
		} `json:"servers"`
	}
)

func (c *Controller) SearchMcpServers(ctx context.Context, in SearchMcpServersInput) (SearchMcpServersOutput, error) {
	userID, ok := FromContext(ctx)
	if !ok {
		return SearchMcpServersOutput{}, fmt.Errorf("missing user ID in context")
	}
	_ = userID

	return SearchMcpServersOutput{}, errors.New("unimplemented")
}

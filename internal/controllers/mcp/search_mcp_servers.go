package mcp

import (
	"context"
)

const (
	searchMcpServersName = "search_mcp_servers"
	searchMcpServersDesc = "Searches for registered public MCP servers using a text query."
)

type (
	SearchMcpServersInput struct {
		Query string `json:"query"           jsonschema:"Search query text"`
		Limit int    `json:"limit,omitempty" jsonschema:"Maximum number of results (default: 10)"`
	}
	SearchMcpServersOutput struct {
		Servers []struct {
			URL  string `json:"url"`
			Name string `json:"name"`
		} `json:"servers"`
	}
)

func (c *Controller) SearchMcpServers(
	ctx context.Context,
	in SearchMcpServersInput,
) (
	SearchMcpServersOutput,
	error,
) {
	userID, ok := FromContext(ctx)
	if !ok {
		return SearchMcpServersOutput{}, ErrUnauthorized
	}

	_ = userID

	return SearchMcpServersOutput{}, ErrUnimplemented
}

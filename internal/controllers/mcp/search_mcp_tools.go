package mcp

import (
	"context"
)

const (
	searchMcpToolsName = "search_mcp_tools"
	searchMcpToolsDesc = "Search for tools across all active MCP accounts by query."
)

type (
	SearchMcpToolsInput struct {
		Query string `json:"query"           jsonschema:"Search query text"`
		Limit int    `json:"limit,omitempty" jsonschema:"Max results"`
	}

	SearchMcpToolsOutput struct {
		Tools []SearchMCPTool `json:"tools"`
	}

	SearchMCPTool struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
)

func (c *Controller) SearchMcpTools(
	ctx context.Context,
	in SearchMcpToolsInput,
) (
	SearchMcpToolsOutput,
	error,
) {
	userID, ok := FromContext(ctx)
	if !ok {
		return SearchMcpToolsOutput{}, ErrUnauthorized
	}

	_ = userID

	return SearchMcpToolsOutput{
		Tools: []SearchMCPTool{},
	}, nil
}

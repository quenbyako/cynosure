package mcp

import (
	"context"
	"fmt"
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
	input SearchMcpToolsInput,
) (
	SearchMcpToolsOutput,
	error,
) {
	userID, ok := FromContext(ctx)
	if !ok {
		return SearchMcpToolsOutput{}, ErrUnauthorized
	}

	matched, err := c.accounts.SearchMcpTools(ctx, userID, input.Query, input.Limit)
	if err != nil {
		return SearchMcpToolsOutput{}, fmt.Errorf("searching mcp tools: %w", err)
	}

	res := make([]SearchMCPTool, len(matched))
	for i, t := range matched {
		res[i] = SearchMCPTool{
			Name:        t.Name(),
			Description: t.Desc(),
		}
	}

	return SearchMcpToolsOutput{Tools: res}, nil
}

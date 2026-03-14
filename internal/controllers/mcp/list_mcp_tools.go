package mcp

import (
	"context"
	"fmt"
)

const (
	listMcpToolsName = "list_mcp_tools"
	listMcpToolsDesc = "Lists all available tools from all active MCP accounts."
)

type (
	Tool struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}

	ListMCPToolsOutput struct {
		Tools []Tool `json:"tools"`
	}
)

func (c *Controller) ListMcpTools(ctx context.Context, _ struct{}) (ListMCPToolsOutput, error) {
	userID, ok := FromContext(ctx)
	if !ok {
		return ListMCPToolsOutput{}, ErrUnauthorized
	}

	accounts, err := c.accounts.ListAccounts(ctx, userID)
	if err != nil {
		return ListMCPToolsOutput{}, fmt.Errorf("listing accounts: %w", err)
	}

	tools := make([]Tool, 0, len(accounts))

	for _, account := range accounts {
		accountTools, err := c.accounts.ListTools(ctx, account.ID())
		if err != nil {
			e := fmt.Errorf("listing tools for account %q: %w", account.ID().ID().String(), err)
			return ListMCPToolsOutput{}, e
		}

		for _, tool := range accountTools {
			tools = append(tools, Tool{
				Name:        tool.Name(),
				Description: tool.Desc(),
			})
		}
	}

	return ListMCPToolsOutput{Tools: tools}, nil
}

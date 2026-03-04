package mcp

import (
	"context"
	"fmt"
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
		return ListMCPToolsOutput{}, fmt.Errorf("missing user ID in context")
	}

	accounts, err := c.accounts.ListAccounts(ctx, userID)
	if err != nil {
		return ListMCPToolsOutput{}, err
	}

	tools := make([]Tool, 0, len(accounts))

	for _, account := range accounts {
		accountTools, err := c.accounts.ListTools(ctx, account.ID())
		if err != nil {
			return ListMCPToolsOutput{}, err
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

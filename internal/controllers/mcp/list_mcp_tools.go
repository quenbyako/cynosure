package mcp

import "context"

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
	userID := userID // TODO: get it from context
	_ = userID

	accounts, err := c.accounts.ListAccounts(ctx, userID)
	if err != nil {
		return ListMCPToolsOutput{}, err
	}

	tools := make([]Tool, 0, len(accounts))

	for _, account := range accounts {
		accountTools, err := c.accounts.ListTools(ctx, account)
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

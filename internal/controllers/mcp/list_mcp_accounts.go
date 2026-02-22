package mcp

import "context"

type (
	ListMcpAccountsOutput struct {
		Accounts []struct {
			AccountID string `json:"account_id"`
			ServerURL string `json:"server_url"`
			Name      string `json:"name"`
		} `json:"accounts"`
	}
)

func (c *Controller) ListMcpAccounts(ctx context.Context, _ struct{}) (ListMcpAccountsOutput, error) {
	userID := userID // TODO: get it from context
	_ = userID

	return ListMcpAccountsOutput{}, nil
}

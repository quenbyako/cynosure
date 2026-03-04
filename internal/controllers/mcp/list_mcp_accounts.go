package mcp

import (
	"context"
	"errors"
)

type (
	ListMcpAccountsOutput struct {
		Accounts []Account `json:"accounts"`
	}

	Account struct {
		AccountID string `json:"account_id"`
		ServerURL string `json:"server_url"`
		Name      string `json:"name"`
	}
)

func (c *Controller) ListMcpAccounts(ctx context.Context, _ struct{}) (ListMcpAccountsOutput, error) {
	userID, ok := FromContext(ctx)
	if !ok {
		return ListMcpAccountsOutput{}, errors.New("unauthorized")
	}

	accounts, err := c.accounts.ListAccounts(ctx, userID)
	if err != nil {
		return ListMcpAccountsOutput{}, err
	}

	result := ListMcpAccountsOutput{
		Accounts: make([]Account, 0, len(accounts)),
	}

	for _, acc := range accounts {
		server, err := c.accounts.GetServerInfo(ctx, acc.ID().Server())
		if err != nil {
			return ListMcpAccountsOutput{}, err
		}

		result.Accounts = append(result.Accounts, Account{
			AccountID: acc.ID().ID().String(),
			ServerURL: server.SSELink().String(),
			Name:      acc.Name(),
		})
	}

	return result, nil
}

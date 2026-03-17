package mcp

import (
	"context"
	"fmt"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
)

const (
	listMcpAccountsName = "list_mcp_accounts"
	listMcpAccountsDesc = "Returns a list of registered and active MCP accounts for the " +
		"current user."
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

func (c *Controller) ListMcpAccounts(
	ctx context.Context, _ struct{},
) (
	ListMcpAccountsOutput,
	error,
) {
	userID, ok := FromContext(ctx)
	if !ok {
		return ListMcpAccountsOutput{}, ErrUnauthorized
	}

	accountList, err := c.accounts.ListAccounts(ctx, userID)
	if err != nil {
		return ListMcpAccountsOutput{}, fmt.Errorf("listing accounts: %w", err)
	}

	result := ListMcpAccountsOutput{
		Accounts: make([]Account, 0, len(accountList)),
	}

	for _, acc := range accountList {
		mapped, err := c.mapAccount(ctx, acc)
		if err != nil {
			return ListMcpAccountsOutput{}, err
		}

		result.Accounts = append(result.Accounts, mapped)
	}

	return result, nil
}

func (c *Controller) mapAccount(
	ctx context.Context,
	acc entities.AccountReadOnly,
) (Account, error) {
	server, err := c.accounts.GetServerInfo(ctx, acc.ID().Server())
	if err != nil {
		e := fmt.Errorf(
			"getting server info for account %q: %w",
			acc.ID().ID().String(), err,
		)

		return Account{}, e
	}

	return Account{
		AccountID: acc.ID().ID().String(),
		ServerURL: server.SSELink().String(),
		Name:      acc.Name(),
	}, nil
}

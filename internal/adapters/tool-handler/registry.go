package primitive

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/k0kubun/pp/v3"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/tools"
)

func (h *Handler) RegisterTools(ctx context.Context, account ids.AccountID, name, description string, token *oauth2.Token) error {
	serverInfo, err := h.servers.GetServerInfo(ctx, account.Server())
	if err != nil {
		return fmt.Errorf("retrieving server info for account %v: %w", account.ID().String(), err)
	}

	httpCLient := http.DefaultClient
	if token != nil {
		// WARN: рефрешер нормально не работает с флоу сохранения, потому что берет
		// контекст из аргумента, и сохраняет его невсегда, то есть резон
		// использовать есть лишь как пример
		//
		// к тому же, он не защищён от гонок, так что надо переделать
		httpCLient = oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))
	}

	c, err := newAsyncClient(ctx, serverInfo.SSELink, httpCLient)

	result, err := c.session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		return fmt.Errorf("listing tools for account %v: %w", account.ID().String(), err)
	}

	collectedTools := make([]tools.ToolInfo, len(result.Tools))
	for i, tool := range result.Tools {
		fmt.Printf("Found tool for account %v: %v\n", account.ID().String(), tool.Name)

		inputSchema, err := json.Marshal(tool.InputSchema)
		if err != nil {
			return fmt.Errorf("marshalling input schema for tool %q: %w", tool.Name, err)
		}
		outputSchema := []byte(`{"type": "string"}`)
		if tool.OutputSchema != nil {
			outputSchema, err = json.Marshal(tool.OutputSchema)
			if err != nil {
				return fmt.Errorf("marshalling output schema for tool %q: %w", tool.Name, err)
			}
		}

		pp.Println("Tool details:", string(inputSchema), string(outputSchema))

		tool, err := tools.NewToolInfo(tool.Name, tool.Description, inputSchema, outputSchema)
		if err != nil {
			return fmt.Errorf("invalid tool %q: %w", tool.Name(), err)
		}

		collectedTools[i] = tool
	}

	opts := []entities.NewAccountOption{
		entities.WithAuthToken(nil),
	}

	if token != nil {
		opts = append(opts, entities.WithAuthToken(token))
	}

	// TODO: здесь ОЧЕНЬ важно применить аналитическую модель, которая будет
	// потом делать корректное описание. Смысл в том, что вся суть крутится
	// вокруг "инструмент по ссылке", и заставлять делать описание нужно лишь в
	// том случае, если даже модель не понимает, зачем нужен этот инструмент и в
	// чем отличие одного аккаунта от другого.
	acc, err := entities.NewAccount(account, name, description, collectedTools, opts...)
	if err != nil {
		return fmt.Errorf("creating account entity: %w", err)
	}

	err = h.accounts.SaveAccount(ctx, acc)
	if err != nil {
		return fmt.Errorf("saving session for account %v: %w", account.ID().String(), err)
	}

	pp.Println("cancelling")
	pp.Println(c.Close())
	pp.Println("cancel done!")

	return nil
}

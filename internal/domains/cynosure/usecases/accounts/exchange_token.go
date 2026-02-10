package accounts

import (
	"context"
	"fmt"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/oauth"
)

func (s *Usecase) ExchangeToken(ctx context.Context, exchangeToken, stateStr string) error {
	ctx, span := s.trace.Start(ctx, "Usecase.ExchangeToken")
	defer span.End()

	if stateStr == "" {
		return fmt.Errorf("state parameter is required")
	}

	if exchangeToken == "" {
		return fmt.Errorf("exchange token is required")
	}

	state, err := oauth.StateFromToken(stateStr, s.key)
	if err != nil {
		return fmt.Errorf("invalid state: %w", err)
	}

	server, err := s.servers.GetServerInfo(ctx, state.Account().Server())
	if err != nil {
		return fmt.Errorf("getting server info: %w", err)
	}

	token, err := s.oauth.Exchange(ctx, server.AuthConfig(), exchangeToken, state.Challenge())
	if err != nil {
		return fmt.Errorf("exchanging token: %w", err)
	}

	account, err := entities.NewAccount(
		state.Account(),
		state.Name(),
		state.Description(),
		entities.WithAuthToken(token),
	)
	if err != nil {
		return fmt.Errorf("constructing account entity: %w", err)
	}

	err = s.accounts.SaveAccount(ctx, account)
	if err != nil {
		return fmt.Errorf("saving account: %w", err)
	}

	rawTools, err := s.toolClient.DiscoverTools(ctx, server.SSELink(), token, state.Account(), state.Description())
	if err != nil {
		return fmt.Errorf("discovering tools: %w", err)
	}

	for _, rawTool := range rawTools {
		var toolID ids.ToolID
		for id := range rawTool.EncodedTools() {
			toolID = id
			break
		}

		// Create tool entity
		// Note: we can't use a closure that captures 'tool' before it exists,
		// so we'll need to refactor this to pass toolID to ExecuteTool instead
		tool, err := entities.NewTool(
			toolID,
			rawTool.Name(),
			rawTool.Desc(),
			rawTool.Params(),
			rawTool.Response(),
		)
		if err != nil {
			return fmt.Errorf("creating tool entity for %q: %w", rawTool.Name(), err)
		}

		// Index the tool and set embedding
		embedding, err := s.index.IndexTool(ctx, tool)
		if err != nil {
			return fmt.Errorf("indexing tool %q: %w", tool.Name(), err)
		}

		tool.SetEmbedding(embedding)

		err = s.tools.SaveTool(ctx, tool)
		if err != nil {
			return fmt.Errorf("saving tool %q: %w", tool.Name(), err)
		}

		tool.ClearEvents()
	}

	return nil
}

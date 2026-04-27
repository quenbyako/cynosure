package accounts

import (
	"context"
	"fmt"

	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/toolclient"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/oauth"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
)

func (s *Usecase) ExchangeToken(ctx context.Context, exchangeToken, stateStr string) error {
	ctx, span := s.trace.Start(ctx, "Usecase.ExchangeToken")
	defer span.End()

	if err := s.validateExchangeParams(exchangeToken, stateStr); err != nil {
		return err
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

	return s.saveExchangedAccountAndQueueTools(ctx, state, token, server)
}

func (s *Usecase) validateExchangeParams(exchangeToken, stateStr string) error {
	if stateStr == "" {
		return fmt.Errorf("%w", ErrStateRequired)
	}

	if exchangeToken == "" {
		return fmt.Errorf("%w", ErrExchangeTokenRequired)
	}

	return nil
}

func (s *Usecase) saveExchangedAccountAndQueueTools(
	ctx context.Context,
	state oauth.State,
	token *oauth2.Token,
	server entities.ServerConfigReadOnly,
) error {
	account, err := entities.NewAccount(
		state.Account(),
		state.Name(),
		state.Description(),
		entities.WithAuthToken(token),
	)
	if err != nil {
		return fmt.Errorf("constructing account entity: %w", err)
	}

	if err := s.accounts.SaveAccount(ctx, account); err != nil {
		return fmt.Errorf("saving account: %w", err)
	}

	ok := s.pool.Submit(ctx, discoveryTask{
		server:  server,
		account: account,
		token:   token,
	})
	if !ok {
		// TODO: add metrics to detect, how many messages were dropped due to non running pool.
		return ErrInternalValidation("failed to submit async request, pool is not working")
	}

	return nil
}

func (s *Usecase) runDiscoveryTask(ctx context.Context, task discoveryTask) {
	ctx, span := s.trace.Start(ctx, "Usecase.ExchangeToken.Discover")
	defer span.End()

	if err := s.saveAccountAndTools(ctx, task.server, task.account, task.token); err != nil {
		span.RecordError(err)
	}
}

func (s *Usecase) saveAccountAndTools(
	ctx context.Context,
	server entities.ServerConfigReadOnly,
	account entities.AccountReadOnly,
	token *oauth2.Token,
) error {
	rawTools, err := s.discoverTools(ctx, server, account, token)
	if err != nil {
		return err
	}

	for _, rawTool := range rawTools {
		if err := s.processAndSaveTool(ctx, account, rawTool); err != nil {
			return err
		}
	}

	return nil
}

func (s *Usecase) discoverTools(
	ctx context.Context,
	server entities.ServerConfigReadOnly,
	account entities.AccountReadOnly,
	token *oauth2.Token,
) ([]tools.RawTool, error) {
	var opts []toolclient.DiscoverToolsOption
	if token != nil {
		opts = append(opts, toolclient.WithAuthToken(token))
	}

	rawTools, err := s.toolClient.DiscoverTools(
		ctx,
		server.SSELink(),
		account.ID(),
		account.Name(),
		account.Description(),
		opts...,
	)
	if err != nil {
		return nil, fmt.Errorf("discovering tools: %w", err)
	}

	return rawTools, nil
}

func (s *Usecase) processAndSaveTool(
	ctx context.Context,
	account entities.AccountReadOnly,
	rawTool tools.RawTool,
) error {
	toolID := s.extractToolID(rawTool)

	tool, err := entities.NewTool(
		toolID,
		account.Name(),
		rawTool.Name(),
		rawTool.Desc(),
		rawTool.Params(),
		rawTool.Response(),
	)
	if err != nil {
		return fmt.Errorf("creating tool entity for %q: %w", rawTool.Name(), err)
	}

	return s.indexAndSaveTool(ctx, tool)
}

func (s *Usecase) indexAndSaveTool(
	ctx context.Context,
	tool *entities.Tool,
) error {
	embedding, err := s.index.IndexTool(ctx, tool)
	if err != nil {
		return fmt.Errorf("indexing tool %q: %w", tool.Name(), err)
	}

	tool.SetEmbedding(embedding)

	if err := s.tools.SaveTool(ctx, tool); err != nil {
		return fmt.Errorf("saving tool %q: %w", tool.Name(), err)
	}

	tool.ClearEvents()

	return nil
}

func (s *Usecase) extractToolID(rawTool tools.RawTool) ids.ToolID {
	var toolID ids.ToolID

	for id := range rawTool.EncodedTools() {
		toolID = id

		break
	}

	return toolID
}

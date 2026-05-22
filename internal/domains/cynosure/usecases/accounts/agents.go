package accounts

import (
	"context"
	"fmt"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/accounts"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

func (s *Usecase) CreateAgent(
	ctx context.Context,
	user ids.UserID,
	modelName string,
	systemPrompt string,
) (ids.AgentID, error) {
	ctx, span := s.trace.Start(ctx, "Usecase.CreateAgent")
	defer span.End()

	agentID, err := ids.RandomAgentID(user)
	if err != nil {
		return ids.AgentID{}, fmt.Errorf("generating random agent ID: %w", err)
	}

	agent, err := entities.NewModelSettings(
		agentID,
		modelName,
		entities.WithSystemMessage(systemPrompt),
	)
	if err != nil {
		return ids.AgentID{}, fmt.Errorf("new model settings: %w", err)
	}

	if err := s.agents.SaveAgent(ctx, agent); err != nil {
		return ids.AgentID{}, fmt.Errorf("saving agent: %w", err)
	}

	return agentID, nil
}

func (s *Usecase) UpdateAgent(
	ctx context.Context,
	user ids.UserID,
	agentID ids.AgentID,
	modelName string,
	systemPrompt string,
) error {
	ctx, span := s.trace.Start(ctx, "Usecase.UpdateAgent")
	defer span.End()

	agent, err := s.agents.GetAgent(ctx, agentID)
	if err != nil {
		return fmt.Errorf("getting agent: %w", err)
	}

	if agent.ID().UserID() != user {
		return ErrAccessDenied
	}

	updated, err := buildUpdatedAgent(agent, modelName, systemPrompt)
	if err != nil {
		return err
	}

	if err := s.agents.SaveAgent(ctx, updated); err != nil {
		return fmt.Errorf("saving updated agent: %w", err)
	}

	return nil
}

func buildUpdatedAgent(
	agent *entities.Agent,
	modelName string,
	systemPrompt string,
) (*entities.Agent, error) {
	model, sysPrompt := getUpdatedFields(agent, modelName, systemPrompt)
	opts := buildAgentOptions(agent, sysPrompt)

	updated, err := entities.NewModelSettings(agent.ID(), model, opts...)
	if err != nil {
		return nil, fmt.Errorf("new model settings: %w", err)
	}

	return updated, nil
}

func getUpdatedFields(
	agent *entities.Agent,
	modelName, systemPrompt string,
) (updatedModel, updatedSystemPrompt string) {
	model := agent.Model()
	if modelName != "" {
		model = modelName
	}

	sysPrompt := agent.SystemMessage()
	if systemPrompt != "" {
		sysPrompt = systemPrompt
	}

	return model, sysPrompt
}

func buildAgentOptions(agent *entities.Agent, sysPrompt string) []entities.NewModelSettingsOption {
	opts := []entities.NewModelSettingsOption{
		entities.WithSystemMessage(sysPrompt),
	}

	if temp, ok := agent.Temperature(); ok {
		opts = append(opts, entities.WithTemperature(temp))
	}

	if topP, ok := agent.TopP(); ok {
		opts = append(opts, entities.WithTopP(topP))
	}

	if maxCtx, ok := agent.MaxContext(); ok {
		opts = append(opts, entities.WithMaxContext(maxCtx))
	}

	if stopWords := agent.StopWords(); len(stopWords) > 0 {
		opts = append(opts, entities.WithStopWords(stopWords))
	}

	return opts
}

func (s *Usecase) DeleteAgent(
	ctx context.Context,
	user ids.UserID,
	agentID ids.AgentID,
) error {
	ctx, span := s.trace.Start(ctx, "Usecase.DeleteAgent")
	defer span.End()

	agent, err := s.agents.GetAgent(ctx, agentID)
	if err != nil {
		return fmt.Errorf("getting agent: %w", err)
	}

	if agent.ID().UserID() != user {
		return ErrAccessDenied
	}

	if err := s.agents.DeleteAgent(ctx, agentID); err != nil {
		return fmt.Errorf("deleting agent: %w", err)
	}

	return nil
}

func (s *Usecase) ListAgents(
	ctx context.Context,
	user ids.UserID,
) ([]*entities.Agent, error) {
	ctx, span := s.trace.Start(ctx, "Usecase.ListAgents")
	defer span.End()

	agents, err := s.agents.ListAgents(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("listing agents: %w", err)
	}

	return agents, nil
}

func (s *Usecase) ReactivateAccountByName(
	ctx context.Context,
	user ids.UserID,
	name string,
) error {
	ctx, span := s.trace.Start(ctx, "Usecase.ReactivateAccountByName")
	defer span.End()

	accResult, err := s.resolveAccount(ctx, user, name)
	if err != nil {
		return err
	}

	return s.reactivateSearchResult(ctx, accResult)
}

func (s *Usecase) reactivateSearchResult(
	ctx context.Context,
	accResult accounts.SearchResult,
) error {
	switch searchRes := accResult.(type) {
	case accounts.SearchResultActive:
		if err := s.accounts.ReactivateAccount(ctx, searchRes.Account.ID()); err != nil {
			return fmt.Errorf("reactivating account: %w", err)
		}

		return s.queueToolDiscovery(ctx, searchRes.Account)
	case accounts.SearchResultDeleted:
		if err := s.accounts.ReactivateAccount(ctx, searchRes.ID()); err != nil {
			return fmt.Errorf("reactivating account: %w", err)
		}

		acc, err := s.accounts.GetAccount(ctx, searchRes.ID())
		if err != nil {
			return fmt.Errorf("getting reactivated account: %w", err)
		}

		return s.queueToolDiscovery(ctx, acc)
	default:
		return fmt.Errorf("unknown account search result type: %T", searchRes)
	}
}

func (s *Usecase) queueToolDiscovery(
	ctx context.Context,
	account *entities.Account,
) error {
	server, err := s.servers.GetServerInfo(ctx, account.ID().Server())
	if err != nil {
		return fmt.Errorf("getting server info: %w", err)
	}

	ok := s.pool.Submit(ctx, discoveryTask{
		server:  server,
		account: account,
		token:   account.Token(),
	})
	if !ok {
		return ErrInternalValidation("failed to submit discovery task, pool is not running")
	}

	return nil
}

func (s *Usecase) DisableAccountByName(
	ctx context.Context,
	user ids.UserID,
	name string,
) error {
	ctx, span := s.trace.Start(ctx, "Usecase.DisableAccountByName")
	defer span.End()

	accResult, err := s.resolveAccount(ctx, user, name)
	if err != nil {
		return err
	}

	if err := s.accounts.DeleteAccount(ctx, accResult.ID()); err != nil {
		return fmt.Errorf("deleting account: %w", err)
	}

	return nil
}

func (s *Usecase) resolveAccount(
	ctx context.Context,
	user ids.UserID,
	name string,
) (accounts.SearchResult, error) {
	accs, err := s.accounts.FindAccountsByName(ctx, user, name)
	if err != nil {
		return nil, fmt.Errorf("finding accounts: %w", err)
	}

	if len(accs) == 0 {
		return nil, accounts.ErrNotFound
	}

	return s.matchServerURL(ctx, accs)
}

func (s *Usecase) matchServerURL(
	ctx context.Context,
	accs []accounts.SearchResult,
) (accounts.SearchResult, error) {
	for _, acc := range accs {
		srv, err := s.servers.GetServerInfo(ctx, acc.ID().Server())
		if err == nil && srv.SSELink() != nil {
			return acc, nil
		}
	}

	return nil, fmt.Errorf("account not found on server: %w", accounts.ErrNotFound)
}

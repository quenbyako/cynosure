package users

import (
	"context"
	"fmt"
	"time"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/toolclient"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

func (s *Usecase) InitializeAccount(ctx context.Context, userID ids.UserID) error {
	// 1. Add/Retrieve Admin MCP Server
	acc, err := s.addAdminMCP(ctx, userID)
	if err != nil {
		return err
	}

	// 2. Discover Tools if none exist
	tools, err := s.tools.ListTools(ctx, acc.ID())
	if err != nil {
		return fmt.Errorf("listing account tools: %w", err)
	}

	if len(tools) == 0 {
		if err := s.discoverTools(ctx, acc); err != nil {
			return fmt.Errorf("discovering tools: %w", err)
		}
	}

	// 3. Create Default Meta-Agent if no agents exist for user
	existingAgents, err := s.agents.ListAgents(ctx, userID)
	if err != nil {
		return fmt.Errorf("listing user agents: %w", err)
	}

	if len(existingAgents) == 0 {
		agentID, err := ids.RandomAgentID(userID)
		if err != nil {
			return fmt.Errorf("generating agent id: %w", err)
		}

		agent, err := entities.NewModelSettings(
			agentID,
			"gemini-2.5-flash", // Default model
			entities.WithSystemMessage(
				"You are Cynosure, a meta-agent assistant. "+
					"You can manage other MCP servers and help user with various tasks.",
			),
		)
		if err != nil {
			return fmt.Errorf("creating default agent entity: %w", err)
		}

		if err := s.agents.SaveAgent(ctx, agent); err != nil {
			return fmt.Errorf("saving default agent: %w", err)
		}
	}

	return nil
}

//nolint:ireturn // returns interface for make entity read-only.
func (s *Usecase) addAdminMCP(
	ctx context.Context,
	userID ids.UserID,
) (entities.AccountReadOnly, error) {
	// 1. Check if admin account already exists
	existingIDs, err := s.accounts.ListAccounts(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("listing user accounts: %w", err)
	}

	for _, accID := range existingIDs {
		if accID.Server() != s.adminMCPID {
			continue
		}

		acc, err := s.accounts.GetAccount(ctx, accID)
		if err != nil {
			return nil, fmt.Errorf("getting admin account: %w", err)
		}

		// Found an existing account. Check if token is still valid.
		if acc.Token() != nil && acc.Token().Expiry.After(time.Now().Add(time.Minute)) {
			return acc, nil
		}

		// If token exists but expired (or missing), we try to issue a new one.
		token, err := s.users.IssueToken(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("refreshing token for admin mcp: %w", err)
		}

		// Update existing account with new token
		updatedAcc, err := entities.NewAccount(
			acc.ID(),
			acc.Name(),
			acc.Description(),
			entities.WithAuthToken(token),
		)
		if err != nil {
			return nil, fmt.Errorf("creating updated account: %w", err)
		}

		if err := s.accounts.SaveAccount(ctx, updatedAcc); err != nil {
			return nil, fmt.Errorf("updating admin account with new token: %w", err)
		}

		return updatedAcc, nil
	}

	// 2. Issue new token
	token, err := s.users.IssueToken(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("issuing token for admin mcp: %w", err)
	}

	accountID, err := ids.RandomAccountID(userID, s.adminMCPID)
	if err != nil {
		return nil, fmt.Errorf("generating account id: %w", err)
	}

	acc, err := entities.NewAccount(
		accountID,
		"Admin Panel",
		"System management interface for Admin MCP",
		entities.WithAuthToken(token),
	)
	if err != nil {
		return nil, fmt.Errorf("creating account entity: %w", err)
	}

	if err := s.accounts.SaveAccount(ctx, acc); err != nil {
		return nil, fmt.Errorf("saving admin account: %w", err)
	}

	return acc, nil
}

func (s *Usecase) discoverTools(ctx context.Context, account entities.AccountReadOnly) error {
	server, err := s.servers.GetServerInfo(ctx, account.ID().Server())
	if err != nil {
		return fmt.Errorf("getting server info: %w", err)
	}

	rawTools, err := s.toolClient.DiscoverTools(
		ctx, server.SSELink(), account.ID(), account.Name(), account.Description(),
		toolclient.WithAuthToken(account.Token()),
	)
	if err != nil {
		return fmt.Errorf("mcp discovery: %w", err)
	}

	for _, rawTool := range rawTools {
		var toolID ids.ToolID
		for id := range rawTool.EncodedTools() {
			toolID = id
			break
		}

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

		embedding, err := s.index.IndexTool(ctx, tool)
		if err != nil {
			return fmt.Errorf("indexing tool %q: %w", tool.Name(), err)
		}

		tool.SetEmbedding(embedding)

		if err := s.tools.SaveTool(ctx, tool); err != nil {
			return fmt.Errorf("saving tool %q: %w", tool.Name(), err)
		}
	}

	return nil
}

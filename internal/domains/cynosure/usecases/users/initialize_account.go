package users

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/toolclient"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
)

func (u *Usecase) InitializeAccount(ctx context.Context, userID ids.UserID) error {
	acc, err := u.addAdminMCP(ctx, userID)
	if err != nil {
		return err
	}

	if err := u.ensureTools(ctx, acc); err != nil {
		return err
	}

	return u.ensureDefaultAgent(ctx, userID)
}

func (u *Usecase) ensureTools(ctx context.Context, acc entities.AccountReadOnly) error {
	toolList, err := u.tools.ListTools(ctx, acc.ID())
	if err != nil {
		return fmt.Errorf("listing account tools: %w", err)
	}

	if len(toolList) == 0 {
		if err := u.discoverTools(ctx, acc); err != nil {
			return fmt.Errorf("discovering tools: %w", err)
		}
	}

	return nil
}

func (u *Usecase) ensureDefaultAgent(ctx context.Context, userID ids.UserID) error {
	existingAgents, err := u.agents.ListAgents(ctx, userID)
	if err != nil {
		return fmt.Errorf("listing user agents: %w", err)
	}

	if len(existingAgents) > 0 {
		return nil
	}

	agentID, err := ids.RandomAgentID(userID)
	if err != nil {
		return fmt.Errorf("generating agent id: %w", err)
	}

	agent, err := entities.NewModelSettings(
		agentID,
		"gemini-2.5-flash", // Default model for now
		entities.WithSystemMessage(
			"You are Cynosure, a meta-agent assistant. "+
				"You can manage other MCP servers and help user with various tasks.",
		),
	)
	if err != nil {
		return fmt.Errorf("creating default agent entity: %w", err)
	}

	if err := u.agents.SaveAgent(ctx, agent); err != nil {
		return fmt.Errorf("saving default agent: %w", err)
	}

	return nil
}

func (u *Usecase) addAdminMCP(
	ctx context.Context,
	userID ids.UserID,
) (entities.AccountReadOnly, error) {
	existingIDs, err := u.accounts.ListAccounts(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("listing user accounts: %w", err)
	}

	for _, accID := range existingIDs {
		if accID.Server() != u.adminMCPID {
			continue
		}

		return u.ensureAdminAccountValid(ctx, userID, accID)
	}

	return u.createAdminAccount(ctx, userID)
}

func (u *Usecase) ensureAdminAccountValid(
	ctx context.Context,
	userID ids.UserID,
	accID ids.AccountID,
) (entities.AccountReadOnly, error) {
	acc, err := u.accounts.GetAccount(ctx, accID)
	if err != nil {
		return nil, fmt.Errorf("getting admin account: %w", err)
	}

	if acc.Token() != nil && acc.Token().Expiry.After(time.Now().Add(time.Minute)) {
		return acc, nil
	}

	token, err := u.users.IssueToken(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("refreshing token for admin mcp: %w", err)
	}

	return u.createAccount(ctx, acc, token)
}

func (u *Usecase) createAccount(
	ctx context.Context,
	acc entities.AccountReadOnly,
	token *oauth2.Token,
) (*entities.Account, error) {
	updatedAcc, err := entities.NewAccount(
		acc.ID(),
		acc.Name(),
		acc.Description(),
		entities.WithAuthToken(token),
	)
	if err != nil {
		return nil, fmt.Errorf("creating updated account: %w", err)
	}

	if err := u.accounts.SaveAccount(ctx, updatedAcc); err != nil {
		return nil, fmt.Errorf("updating admin account with new token: %w", err)
	}

	return updatedAcc, nil
}

func (u *Usecase) createAdminAccount(
	ctx context.Context,
	userID ids.UserID,
) (entities.AccountReadOnly, error) {
	token, err := u.users.IssueToken(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("issuing token for admin mcp: %w", err)
	}

	accountID, err := ids.RandomAccountID(userID, u.adminMCPID)
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

	if err := u.accounts.SaveAccount(ctx, acc); err != nil {
		return nil, fmt.Errorf("saving admin account: %w", err)
	}

	return acc, nil
}

func (u *Usecase) discoverTools(ctx context.Context, account entities.AccountReadOnly) error {
	server, err := u.servers.GetServerInfo(ctx, account.ID().Server())
	if err != nil {
		return fmt.Errorf("getting server info: %w", err)
	}

	rawTools, err := u.toolClient.DiscoverTools(
		ctx, server.SSELink(), account.ID(), account.Name(), account.Description(),
		toolclient.WithAuthToken(account.Token()),
	)
	if err != nil {
		return fmt.Errorf("mcp discovery: %w", err)
	}

	for _, rawTool := range rawTools {
		if err := u.importTool(ctx, account, rawTool); err != nil {
			return err
		}
	}

	return nil
}

func (u *Usecase) importTool(
	ctx context.Context,
	account entities.AccountReadOnly,
	rawTool tools.RawTool,
) error {
	var toolID ids.ToolID
	for id := range rawTool.EncodedTools() {
		toolID = id
		break
	}

	tool, err := u.newToolWithEmbedding(ctx, toolID, account, rawTool)
	if err != nil {
		return err
	}

	if err := u.tools.SaveTool(ctx, tool); err != nil {
		return fmt.Errorf("saving tool %q: %w", tool.Name(), err)
	}

	return nil
}

func (u *Usecase) newToolWithEmbedding(
	ctx context.Context,
	toolID ids.ToolID,
	account entities.AccountReadOnly,
	rawTool tools.RawTool,
) (*entities.Tool, error) {
	tool, err := entities.NewTool(
		toolID,
		account.Name(),
		rawTool.Name(),
		rawTool.Desc(),
		rawTool.Params(),
		rawTool.Response(),
	)
	if err != nil {
		return nil, fmt.Errorf("creating tool entity for %q: %w", rawTool.Name(), err)
	}

	embedding, err := u.index.IndexTool(ctx, tool)
	if err != nil {
		return nil, fmt.Errorf("indexing tool %q: %w", tool.Name(), err)
	}

	tool.SetEmbedding(embedding)

	// cleaning events, cause it's newly created tool
	tool.ClearEvents()

	return tool, nil
}

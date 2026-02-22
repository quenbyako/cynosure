package mcp

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

type (
	AuthorizeMcpServerInput struct {
		URL string `json:"url" jsonschema:"The full URL of the MCP server to authorize"`
		// TODO: move pattern definition to schema. It's just a temporary solution.
		Name        string `json:"name" jsonschema:"The name of the account that will be identified by. CRITICAL: Only [a-zA-Z0-9_-] allowed, no spaces. Example: 'my-server-01'"`
		Description string `json:"description" jsonschema:"Description of the account. Note, that it will be emedded and used as a context for the agent. Must not be empty."`
	}
	AuthorizeMcpServerOutput struct {
		Connected     bool   `json:"connected" jsonschema:"Whether the server is already connected"`
		AuthURL       string `json:"auth_url,omitempty" jsonschema:"Link for authorization if connected=false"`
		TempAccountID string `json:"temp_account_id,omitempty" jsonschema:"ID of the account to be created after auth"`
		AccountID     string `json:"account_id,omitempty" jsonschema:"Final account ID if connected=true"`
		ValidUntil    string `json:"valid_until,omitempty" jsonschema:"Expiration time of the auth link"`
	}
)

// TODO: this MUST be a usecase, there is too much of logic here
func (c *Controller) AuthorizeMcpServer(ctx context.Context, in AuthorizeMcpServerInput) (AuthorizeMcpServerOutput, error) {
	userID := userID // TODO: get it from context

	serverURL, err := url.Parse(in.URL)
	if err != nil {
		return AuthorizeMcpServerOutput{}, fmt.Errorf("invalid server URL: %w", err)
	}

	id, authRequired, err := c.servers.AddServer(ctx, serverURL)
	if err != nil {
		return AuthorizeMcpServerOutput{}, fmt.Errorf("failed to register server: %w", err)
	}
	if !authRequired {
		return AuthorizeMcpServerOutput{
			Connected:     true,
			AccountID:     id.ID().String(),
			AuthURL:       "",
			TempAccountID: "",
			ValidUntil:    "",
		}, nil
	}

	// next is auth link

	link, accountID, validUntil, err := c.accounts.SetupAuthLink(ctx, id, userID, in.Name, in.Description)
	if err != nil {
		return AuthorizeMcpServerOutput{}, fmt.Errorf("failed to setup auth link: %w", err)
	}

	return AuthorizeMcpServerOutput{
		Connected:     false,
		AuthURL:       link.String(),
		TempAccountID: accountID.ID().String(),
		AccountID:     "",
		ValidUntil:    validUntil.Format(time.RFC3339),
	}, nil
}

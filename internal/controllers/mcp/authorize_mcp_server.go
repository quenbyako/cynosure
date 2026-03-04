package mcp

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/accounts"
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
		ValidUntil    string `json:"valid_until,omitempty" jsonschema:"Expiration time of the auth link"`

		AccountID     string `json:"account_id,omitempty" jsonschema:"Final account ID if connected=true"`
		AuthAvailable bool   `json:"auth_available,omitempty" jsonschema:"If true, anonymous connection is already created, but user may authorize by request"`
	}
)

// TODO: this MUST be a usecase, there is too much of logic here
func (c *Controller) AuthorizeMcpServer(ctx context.Context, in AuthorizeMcpServerInput) (AuthorizeMcpServerOutput, error) {
	userID, ok := FromContext(ctx)
	if !ok {
		return AuthorizeMcpServerOutput{}, fmt.Errorf("missing user ID in context")
	}

	serverURL, err := url.Parse(in.URL)
	if err != nil {
		return AuthorizeMcpServerOutput{}, fmt.Errorf("invalid server URL: %w", err)
	}

	if in.Name == "" {
		return AuthorizeMcpServerOutput{}, fmt.Errorf("name is required")
	}

	if in.Description == "" {
		return AuthorizeMcpServerOutput{}, fmt.Errorf("description is required")
	}

	fmt.Printf("called %q %q %q\n", in.URL, in.Name, in.Description)

	// next is auth link

	resp, err := c.accounts.AddAccount(ctx, userID, serverURL, in.Name, in.Description)
	if err != nil {
		return AuthorizeMcpServerOutput{}, fmt.Errorf("failed to setup auth link: %w", err)
	}

	switch resp := resp.(type) {
	case accounts.AddAccountResponseAuthRequired:
		return AuthorizeMcpServerOutput{
			Connected:     false,
			AuthURL:       resp.AuthURL().String(),
			TempAccountID: resp.TempAccountID().ID().String(),
			AccountID:     "",
			ValidUntil:    resp.ValidUntil().Format(time.RFC3339),
		}, nil
	case accounts.AddAccountResponseOK:
		return AuthorizeMcpServerOutput{
			Connected:     true,
			AuthURL:       "",
			TempAccountID: "",
			AccountID:     resp.AccountID().ID().String(),
			ValidUntil:    "",
		}, nil
	default:
		panic(fmt.Sprintf("unexpected accounts.AddAccountResponse: %#v", resp))
	}
}

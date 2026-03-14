package mcp

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/accounts"
)

const (
	authorizeMcpServerName = "authorize_mcp_server"
	authorizeMcpServerDesc = "Registers an MCP server by URL. Returns either an auth link or a " +
		"direct account ID."
)

//nolint:lll // unfortunately, we have to ignore long lines, cause we must define whole docstring.
type (
	AuthorizeMcpServerInput struct {
		URL string `json:"url" jsonschema:"The full URL of the MCP server to authorize"`
		// TODO: move pattern definition to schema. It's just a temporary solution.
		Name        string `json:"name"        jsonschema:"The name of the account that will be identified by. CRITICAL: Only [a-zA-Z0-9_-] allowed, no spaces. Example: 'my-server-01'"`
		Description string `json:"description" jsonschema:"Description of the account. Note, that it will be emedded and used as a context for the agent. Must not be empty."`
	}
	AuthorizeMcpServerOutput struct {
		AuthURL       string `json:"auth_url,omitempty"        jsonschema:"Link for authorization if connected=false"`
		TempAccountID string `json:"temp_account_id,omitempty" jsonschema:"ID of the account to be created after auth"`
		ValidUntil    string `json:"valid_until,omitempty"     jsonschema:"Expiration time of the auth link"`
		AccountID     string `json:"account_id,omitempty"      jsonschema:"Final account ID if connected=true"`
		Connected     bool   `json:"connected"                 jsonschema:"Whether the server is already connected"`
		AuthAvailable bool   `json:"auth_available,omitempty"  jsonschema:"If true, anonymous connection is already created, but user may authorize by request"`
	}
)

func (c *Controller) validateAuthorizeInput(input AuthorizeMcpServerInput) (*url.URL, error) {
	serverURL, err := url.Parse(input.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid server URL: %w", err)
	}

	if input.Name == "" {
		return nil, ErrNameRequired
	}

	if input.Description == "" {
		return nil, ErrDescriptionRequired
	}

	return serverURL, nil
}

// AuthorizeMcpServer registers an MCP server by URL.
func (c *Controller) AuthorizeMcpServer(
	ctx context.Context,
	input AuthorizeMcpServerInput,
) (
	AuthorizeMcpServerOutput,
	error,
) {
	userID, ok := FromContext(ctx)
	if !ok {
		return AuthorizeMcpServerOutput{}, ErrUnauthorized
	}

	serverURL, err := c.validateAuthorizeInput(input)
	if err != nil {
		return AuthorizeMcpServerOutput{}, err
	}

	fmt.Printf("called %q %q %q\n", input.URL, input.Name, input.Description)

	// next is auth link

	resp, err := c.accounts.AddAccount(ctx, userID, serverURL, input.Name, input.Description)
	if err != nil {
		return AuthorizeMcpServerOutput{}, fmt.Errorf("failed to setup auth link: %w", err)
	}

	return c.mapAddAccountResponse(resp)
}

func (c *Controller) mapAddAccountResponse(resp any) (AuthorizeMcpServerOutput, error) {
	switch respType := resp.(type) {
	case accounts.AddAccountResponseAuthRequired:
		return AuthorizeMcpServerOutput{
			AuthURL:       respType.AuthURL().String(),
			TempAccountID: respType.TempAccountID().ID().String(),
			ValidUntil:    respType.ValidUntil().Format(time.RFC3339),
			AccountID:     "",
			Connected:     false,
			AuthAvailable: false,
		}, nil
	case accounts.AddAccountResponseOK:
		return AuthorizeMcpServerOutput{
			AuthURL:       "",
			TempAccountID: "",
			ValidUntil:    "",
			AccountID:     respType.AccountID().ID().String(),
			Connected:     true,
			AuthAvailable: false,
		}, nil
	default:
		return AuthorizeMcpServerOutput{}, fmt.Errorf(
			"%w: %#v", ErrUnexpectedAccountResponse, respType,
		)
	}
}

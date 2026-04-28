package oauth

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/quenbyako/cynosure/contrib/oauth-openapi/gen/go/oauth"
	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/oauthhandler"
)

// RegisterClient performs dynamic OAuth 2.0 client registration for the given
// origin URL. The process follows three steps:
//
// Step 1 — discover protected resource metadata (RFC 9728):
//  1. try the suggested protected resource URL (if any)
//  2. try granular path  <origin>/.well-known/oauth-protected-resource/<path>
//  3. try domain-level  <origin>/.well-known/oauth-protected-resource
//  4. fall back to treating the origin hostname as the authorization server
//
// Step 2 — fetch authorization server metadata (RFC 8414):
//  1. request <auth-server>/.well-known/oauth-authorization-server
//  2. extract registration, authorization and token endpoints
//
// Step 3 — register the client (RFC 7591):
//  1. POST to the registration endpoint
//  2. return the resulting oauth2.Config and optional expiry time
func (h *Handler) RegisterClient(
	ctx context.Context, originURL *url.URL, clientName string, redirect *url.URL,
	opts ...oauthhandler.RegisterClientOption,
) (*oauth2.Config, time.Time, error) {
	switch {
	case originURL == nil:
		return nil, time.Time{}, errInternalValidation("origin url is nil")
	case clientName == "":
		return nil, time.Time{}, errInternalValidation("client name is empty")
	case redirect == nil:
		return nil, time.Time{}, errInternalValidation("redirect url is nil")
	}

	params := oauthhandler.RegisterClientParams(opts...)

	client := h.externalClient
	if params.Internal() {
		client = h.internalClient
	}

	oauthClient, err := oauth.NewClientWithResponses("", oauth.WithHTTPClient(client))
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to create oauth client: %w", err)
	}

	return h.runRegistration(
		ctx, oauthClient, originURL, clientName, redirect, params.SuggestedProtectedResource(),
	)
}

// runRegistration sequences the three registration steps and wires their
// outputs together.
func (h *Handler) runRegistration(
	ctx context.Context,
	client oauth.ClientWithResponsesInterface,
	originURL *url.URL,
	clientName string,
	redirect *url.URL,
	suggestedURL *url.URL,
) (*oauth2.Config, time.Time, error) {
	// Step 1 — protected resource metadata → authorization server list
	authInfo, err := h.discoverAuthServers(ctx, client, originURL, suggestedURL)
	if err != nil {
		return nil, time.Time{}, err
	}

	// Step 2 — authorization server metadata → endpoints + scopes
	serverMeta, err := h.resolveAuthMetadata(
		ctx, client, authInfo.authServers, authInfo.resourceDocURL,
	)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to get authorization server metadata: %w", err)
	}

	// Step 3 — register client → oauth2.Config
	return h.registerOAuthClient(
		ctx, client, clientName, redirect, authInfo.resourceScopes, serverMeta,
	)
}

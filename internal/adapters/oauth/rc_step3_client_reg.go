package oauth

// rc_step3_client_reg.go — Step 3 of dynamic client registration.
//
// Goal: register a new OAuth 2.0 client at the authorization server's dynamic
// client registration endpoint (RFC 7591) and return a ready-to-use
// oauth2.Config.
//
// Process:
//  1. Determine the final set of scopes to request:
//     resource scopes  >  auth server scopes  >  handler defaults.
//  2. POST a minimal ClientRegistration request to the registration endpoint.
//     We only fill required and MCP-relevant fields; optional fields are
//     left nil so the server can apply its own defaults.
//  3. Parse the response:
//     - non-2xx  → RegistrationError (typed, callers can inspect status+body)
//     - 2xx      → extract client_id, client_secret, expiry
//  4. Build and return the oauth2.Config.
//
// Artifacts returned to the caller:
//   - *oauth2.Config  ready for the authorization code flow
//   - time.Time       when the client secret expires (zero = no expiry)

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/quenbyako/cynosure/contrib/oauth-openapi/gen/go/oauth"
	"golang.org/x/oauth2"
)

// registerOAuthClient executes Step 3.
func (h *Handler) registerOAuthClient(
	ctx context.Context,
	client oauth.ClientWithResponsesInterface,
	clientName string,
	redirect *url.URL,
	resourceScopes []string,
	serverMeta serverMetaResult,
) (*oauth2.Config, time.Time, error) {
	finalScopes := getAny(resourceScopes, serverMeta.authScopes, h.defaultScopes)

	clientData, expiresAt, err := postClientRegistration(
		ctx, client, clientName, redirect, serverMeta.regEndpoint, finalScopes,
	)
	if err != nil {
		return nil, time.Time{}, err
	}

	cfg := buildOAuthConfig(
		clientData,
		serverMeta.authEndpoint, serverMeta.tokenEndpoint,
		redirect, finalScopes,
	)

	return cfg, expiresAt, nil
}

// postClientRegistration sends the registration request and parses the
// response, breaking the work into two focused functions.
func postClientRegistration(
	ctx context.Context,
	client oauth.ClientWithResponsesInterface,
	clientName string,
	redirect, regEndpoint *url.URL,
	finalScopes []string,
) (*oauth.ClientRegistrationResponse, time.Time, error) {
	registerResp, err := sendClientRegistrationRequest(
		ctx, client, clientName, redirect, regEndpoint, finalScopes,
	)
	if err != nil {
		return nil, time.Time{}, err
	}

	return parseClientRegistrationResponse(registerResp, regEndpoint)
}

// sendClientRegistrationRequest builds and POSTs the RFC 7591 request body.
// Only the fields required by MCP (grant types, redirect URIs, scope) are
// set; the server fills in everything else via defaults.
func sendClientRegistrationRequest(
	ctx context.Context,
	client oauth.ClientWithResponsesInterface,
	clientName string,
	redirect, regEndpoint *url.URL,
	finalScopes []string,
) (*oauth.RegisterOauthClientResponse, error) {
	grantTypes := []string{"authorization_code", "refresh_token"}
	responseTypes := []string{"code"}
	tokenEndpointAuthMethod := "client_secret_basic"
	scope := strings.Join(finalScopes, " ")

	//nolint:exhaustruct // there is too many optional fields
	body := oauth.RegisterOauthClientJSONRequestBody{
		ClientName:              clientName,
		RedirectUris:            []string{redirect.String()},
		TokenEndpointAuthMethod: &tokenEndpointAuthMethod,
		GrantTypes:              &grantTypes,
		ResponseTypes:           &responseTypes,
		Scope:                   &scope,
	}

	resp, err := client.RegisterOauthClientWithResponse(ctx, "", body, useExactURI(regEndpoint))
	if err != nil {
		return nil, fmt.Errorf("failed to register client: %w", err)
	}

	return resp, nil
}

// parseClientRegistrationResponse validates the HTTP status, extracts
// client credentials and the optional expiry, and returns a typed
// RegistrationError on non-2xx so callers can inspect the server's body.
func parseClientRegistrationResponse(
	registerResp *oauth.RegisterOauthClientResponse,
	regEndpoint *url.URL,
) (*oauth.ClientRegistrationResponse, time.Time, error) {
	code := registerResp.StatusCode()
	if code != http.StatusCreated && code != http.StatusOK {
		return nil, time.Time{}, &RegistrationError{
			StatusCode: code,
			Endpoint:   regEndpoint.String(),
			Body:       string(registerResp.Body),
		}
	}

	clientData := registerResp.JSONDefault
	if clientData == nil {
		return nil, time.Time{}, errInternalValidation("empty response when registering client")
	}

	var expiresAt time.Time
	if clientData.ClientSecretExpiresAt != nil && *clientData.ClientSecretExpiresAt > 0 {
		expiresAt = time.Unix(int64(*clientData.ClientSecretExpiresAt), 0)
	}

	return clientData, expiresAt, nil
}

// buildOAuthConfig assembles the oauth2.Config from the registration response
// and the endpoints we collected in Step 2.
func buildOAuthConfig(
	clientData *oauth.ClientRegistrationResponse,
	authEndpoint, tokenEndpoint *url.URL,
	redirect *url.URL,
	finalScopes []string,
) *oauth2.Config {
	var clientSecret, authURL, tokenURL string

	if clientData.ClientSecret != nil {
		clientSecret = *clientData.ClientSecret
	}

	if authEndpoint != nil {
		authURL = authEndpoint.String()
	}

	if tokenEndpoint != nil {
		tokenURL = tokenEndpoint.String()
	}

	//nolint:exhaustruct // DeviceAuthURL not used in authorization code flow
	return &oauth2.Config{
		ClientID:     clientData.ClientId,
		ClientSecret: clientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:   authURL,
			TokenURL:  tokenURL,
			AuthStyle: oauth2.AuthStyleAutoDetect,
		},
		RedirectURL: redirect.String(),
		Scopes:      finalScopes,
	}
}

func getAny(slices ...[]string) []string {
	for _, val := range slices {
		if len(val) > 0 {
			return val
		}
	}

	return nil
}

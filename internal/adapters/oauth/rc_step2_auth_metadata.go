package oauth

// rc_step2_auth_metadata.go — Step 2 of dynamic client registration.
//
// Goal: obtain authorization server metadata (RFC 8414) for every candidate
// server discovered in Step 1, and extract the endpoints needed for Step 3.
//
// Process:
//  1. For each auth server URL, request
//     <server>/.well-known/oauth-authorization-server[/<path>].
//  2. Parse the JSON response to extract:
//     - registration endpoint (mandatory — if absent the server doesn't
//       support dynamic registration, which is a hard error)
//     - authorization endpoint
//     - token endpoint
//     - supported scopes
//  3. Return on the first server that responds successfully.
//
// Artifacts for Step 3:
//   - regEndpoint    where to POST the client registration request
//   - authEndpoint   for building the oauth2.Config
//   - tokenEndpoint  for building the oauth2.Config
//   - authScopes     scopes advertised by the authorization server

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/quenbyako/cynosure/contrib/oauth-openapi/gen/go/oauth"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/oauthhandler"
)

// serverMetaResult carries the output of Step 2.
type serverMetaResult struct {
	regEndpoint   *url.URL
	authEndpoint  *url.URL
	tokenEndpoint *url.URL
	authScopes    []string
}

// resolveAuthMetadata executes Step 2: tries each candidate auth server in
// order and returns the result of the first one that succeeds.
func (h *Handler) resolveAuthMetadata(
	ctx context.Context,
	client oauth.ClientWithResponsesInterface,
	authServers []*url.URL,
	resourceDocURL *url.URL,
) (serverMetaResult, error) {
	var lastErr error

	for _, authServer := range authServers {
		result, err := fetchServerMetadata(ctx, client, authServer, resourceDocURL)
		if err == nil {
			return result, nil
		}

		lastErr = err
	}

	return serverMetaResult{}, lastErr
}

// fetchServerMetadata fetches and validates RFC 8414 metadata for a single
// authorization server.
func fetchServerMetadata(
	ctx context.Context,
	client oauth.ClientWithResponsesInterface,
	authServer *url.URL,
	resourceDocURL *url.URL,
) (serverMetaResult, error) {
	metadataResp, err := client.GetAuthServerMetadataWithResponse(
		ctx, makeWellKnownEditor(authServer),
	)
	if err != nil {
		return serverMetaResult{}, fmt.Errorf("fetching auth server metadata: %w", err)
	}

	if metadataResp.StatusCode() != http.StatusOK {
		return serverMetaResult{}, errInternalValidation(
			"unexpected status code %d when requesting %s",
			metadataResp.StatusCode(), authServer.String(),
		)
	}

	metadata := metadataResp.JSON200
	if metadata == nil {
		return serverMetaResult{}, errInternalValidation("empty metadata response")
	}

	return parseAuthServerMetadata(metadata, resourceDocURL)
}

// parseAuthServerMetadata extracts registration/auth/token endpoints from the
// RFC 8414 response. Returns ErrDynamicClientRegistrationNotSupported when the
// server omits the registration endpoint.
func parseAuthServerMetadata(
	metadata *oauth.AuthorizationServerMetadata,
	resourceDocURL *url.URL,
) (serverMetaResult, error) {
	if metadata.RegistrationEndpoint == nil {
		// If a resource documentation URL was discovered in Step 1 we
		// include it in the error so callers can show users where to find
		// manual registration instructions.
		return serverMetaResult{},
			oauthhandler.ErrDynamicClientRegistrationNotSupported(resourceDocURL)
	}

	regEndpoint, err := url.Parse(*metadata.RegistrationEndpoint)
	if err != nil {
		return serverMetaResult{}, fmt.Errorf("failed to parse registration endpoint: %w", err)
	}

	//nolint:exhaustruct // optional fields filled conditionally by fillAuthServerEndpoints
	result := serverMetaResult{regEndpoint: regEndpoint}

	if err = fillAuthServerEndpoints(&result, metadata); err != nil {
		return serverMetaResult{}, err
	}

	if metadata.ScopesSupported != nil {
		result.authScopes = *metadata.ScopesSupported
	}

	return result, nil
}

// fillAuthServerEndpoints parses the optional authz and token endpoint URLs
// into the result struct.
func fillAuthServerEndpoints(
	result *serverMetaResult,
	metadata *oauth.AuthorizationServerMetadata,
) error {
	var err error

	if metadata.AuthorizationEndpoint != nil {
		if result.authEndpoint, err = url.Parse(*metadata.AuthorizationEndpoint); err != nil {
			return fmt.Errorf("failed to parse authorization endpoint: %w", err)
		}
	}

	if metadata.TokenEndpoint != nil {
		if result.tokenEndpoint, err = url.Parse(*metadata.TokenEndpoint); err != nil {
			return fmt.Errorf("failed to parse token endpoint: %w", err)
		}
	}

	return nil
}

// makeWellKnownEditor returns a request editor that rewrites the URL to the
// RFC 8414 well-known path for the given auth server. The path suffix of the
// server URL is appended so that path-bound metadata configurations work.
func makeWellKnownEditor(authServer *url.URL) oauth.RequestEditorFn {
	return func(ctx context.Context, req *http.Request) error {
		cloned := *authServer
		if !strings.HasPrefix(cloned.Path, "/.well-known/oauth-authorization-server") {
			cloned.Path = path.Join("/.well-known/oauth-authorization-server", cloned.Path)
		}

		req.URL = &cloned
		req.Host = cloned.Host

		return nil
	}
}

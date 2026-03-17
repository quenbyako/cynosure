package oauth

// rc_step1_discover_resource.go — Step 1 of dynamic client registration.
//
// Goal: locate the authorization server(s) for a given resource URL.
//
// Discovery order (per RFC 9728 + MCP OAuth 2.1 draft):
//  1. Use the caller-supplied suggestedURL directly.
//  2. Try <origin>/.well-known/oauth-protected-resource/<original-path>
//     (granular — some MCP servers expose distinct metadata per tool path).
//  3. Try <origin>/.well-known/oauth-protected-resource
//     (standard domain-level endpoint).
//  4. If all three fail — fall back to treating the origin host itself as the
//     authorization server. This matches servers that don't implement RFC 9728.
//
// Artifacts produced for the next step:
//   - authServers     list of authorization server URLs to try (in order)
//   - resourceDocURL  optional link shown to the user when things go wrong
//   - resourceScopes  optional scopes advertised by the resource server

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path"

	"github.com/quenbyako/cynosure/contrib/oauth-openapi/gen/go/oauth"
)

// authDiscoveryResult carries the output of Step 1.
type authDiscoveryResult struct {
	authServers    []*url.URL
	resourceDocURL *url.URL
	resourceScopes []string
}

// discoverAuthServers executes Step 1.
func (h *Handler) discoverAuthServers(
	ctx context.Context,
	client oauth.ClientWithResponsesInterface,
	originURL, suggestedURL *url.URL,
) (authDiscoveryResult, error) {
	metadata, err := discoverProtectedResourceMetadata(ctx, client, originURL, suggestedURL)
	if err != nil {
		// Fallback (step 4 above): when no metadata endpoint responds, we trust
		// the origin host as the authorization server. This is intentional — the
		// error from discovery is not propagated.
		//nolint:nilerr // intentional fallback on any discovery failure
		return authDiscoveryResult{
			authServers:    []*url.URL{{Scheme: originURL.Scheme, Host: originURL.Host}},
			resourceScopes: nil,
			resourceDocURL: nil,
		}, nil
	}

	if metadata == nil {
		return authDiscoveryResult{}, errInternalValidation("empty metadata response")
	}

	return parseProtectedResourceMetadata(metadata)
}

// parseProtectedResourceMetadata extracts the fields we care about from the
// RFC 9728 response.
func parseProtectedResourceMetadata(
	metadata *oauth.ProtectedResourceMetadata,
) (authDiscoveryResult, error) {
	var result authDiscoveryResult

	if authSrv := metadata.AuthorizationServers; authSrv != nil {
		for _, srvRaw := range *authSrv {
			srv, err := url.Parse(srvRaw)
			if err != nil {
				return authDiscoveryResult{}, fmt.Errorf(
					"failed to parse authorization server URL: %w", err,
				)
			}

			result.authServers = append(result.authServers, srv)
		}
	}

	if doc := metadata.ResourceDocumentation; doc != nil {
		parsed, err := url.Parse(*doc)
		if err != nil {
			return authDiscoveryResult{}, fmt.Errorf(
				"failed to parse resource documentation URL: %w", err,
			)
		}

		result.resourceDocURL = parsed
	}

	return result, nil
}

// discoverProtectedResourceMetadata tries the four probe URLs in order and
// returns the first successful response.
//
// TODO: immediately return on infrastructure errors (5xx, network timeouts)
// rather than silently continuing to the next probe URL.
func discoverProtectedResourceMetadata(
	ctx context.Context,
	client oauth.ClientWithResponsesInterface,
	originURL, suggestedURL *url.URL,
) (*oauth.ProtectedResourceMetadata, error) {
	// Probe 1: caller-supplied hint
	if suggestedURL != nil {
		if meta, ok := tryGetResourceMetadata(ctx, client, useExactURI(suggestedURL)); ok {
			return meta, nil
		}
	}

	if originURL == nil {
		return nil, errInternalValidation("origin URL is required to discover metadata")
	}

	// Probe 2: granular path
	if meta, ok := tryGetResourceMetadata(
		ctx, client, useGranularProtectedResourceEndpoint(originURL),
	); ok {
		return meta, nil
	}

	// Probe 3: domain-level
	if meta, ok := tryGetResourceMetadata(
		ctx, client, useDomainProtectedResourceEndpoint(originURL),
	); ok {
		return meta, nil
	}

	return nil, errInternalValidation(
		"failed to discover protected resource metadata for %s",
		originURL.String(),
	)
}

// tryGetResourceMetadata fires a single metadata request and returns (meta,
// true) on success or (nil, false) on any error or non-2xx/3xx response.
func tryGetResourceMetadata(
	ctx context.Context,
	client oauth.ClientWithResponsesInterface,
	editor oauth.RequestEditorFn,
) (*oauth.ProtectedResourceMetadata, bool) {
	resp, err := client.GetResourceMetadataWithResponse(ctx, editor)
	if err != nil || resp.StatusCode() >= http.StatusBadRequest {
		return nil, false
	}

	return resp.JSONDefault, resp.JSONDefault != nil
}

// --- request editors ---------------------------------------------------------

func useExactURI(uri *url.URL) oauth.RequestEditorFn {
	return func(ctx context.Context, req *http.Request) error {
		req.URL = uri
		req.Host = uri.Host

		return nil
	}
}

func useGranularProtectedResourceEndpoint(originURL *url.URL) oauth.RequestEditorFn {
	return func(ctx context.Context, req *http.Request) error {
		cloned := *originURL
		cloned.Path = path.Join("/.well-known/oauth-protected-resource", originURL.Path)

		req.URL = &cloned
		req.Host = cloned.Host

		return nil
	}
}

func useDomainProtectedResourceEndpoint(originURL *url.URL) oauth.RequestEditorFn {
	return func(ctx context.Context, req *http.Request) error {
		cloned := *originURL
		cloned.Path = "/.well-known/oauth-protected-resource"

		req.URL = &cloned
		req.Host = cloned.Host

		return nil
	}
}

package oauth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/quenbyako/cynosure/contrib/oauth-openapi/gen/go/oauth"
	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/oauthhandler"
)

func (h *Handler) RegisterClient(ctx context.Context, originURL *url.URL, clientName string, redirect *url.URL, opts ...oauthhandler.RegisterClientOption) (cfg *oauth2.Config, expiresAt time.Time, err error) {
	p := oauthhandler.RegisterClientParams(opts...)

	if originURL == nil {
		return nil, time.Time{}, fmt.Errorf("origin url is nil")
	}
	if clientName == "" {
		return nil, time.Time{}, fmt.Errorf("client name is empty")
	}
	if redirect == nil {
		return nil, time.Time{}, fmt.Errorf("redirect url is nil")
	}

	client, err := oauth.NewClientWithResponses("", oauth.WithHTTPClient(h.client))
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to create oauth client: %w", err)
	}

	// Step 1: discovering protected resource metadata, this is necessary to get
	// authorization server url
	//
	// 1. try suggested protected resource
	// 2. if fails — try granular request to /oauth-protected-resource/mcp/whatever, based on initial request URL
	// 3. if fails — use default endpoint <request host>/.well-known/oauth-protected-resource
	// 4. if fails — use hostname as authorization server url.
	//
	// artifacts on this step:
	// 1. authorization server METADATA url
	// 2. mcp server short name (optional)
	// 3. resource documentation (optional), will be used when throw next errors.

	var (
		authServers              []*url.URL
		serviceServerShortName   string
		resourceDocumentationURL *url.URL
		resourceScopes           []string
	)

	if metadata, err := discoverProtectedResourceMetadata(ctx, client, originURL, p.SuggestedProtectedResource()); err == nil {
		if metadata == nil {
			return nil, time.Time{}, fmt.Errorf("empty metadata response")
		}
		if authSrv := metadata.AuthorizationServers; authSrv != nil {
			for _, srvRaw := range *authSrv {
				srv, parseErr := url.Parse(srvRaw)
				if parseErr != nil {
					return nil, time.Time{}, fmt.Errorf("failed to parse authorization server URL: %w", parseErr)
				}
				authServers = append(authServers, srv)
			}
		}
		if name := metadata.ResourceName; name != nil {
			serviceServerShortName = *name
		}
		if doc := metadata.ResourceDocumentation; doc != nil {
			if resourceDocumentationURL, err = url.Parse(*doc); err != nil {
				return nil, time.Time{}, fmt.Errorf("failed to parse resource documentation URL: %w", err)
			}
		}
	} else {
		// fallback: if we didn't find authorization server url, we have to
		// trust domain.
		authServers = []*url.URL{
			{Scheme: originURL.Scheme, Host: originURL.Host},
		}
	}

	// TODO: return to caller this data. It will be extremely useful for adding
	// data to server as default name to server.
	_ = serviceServerShortName

	// Step 2: getting server authorization metadata.
	//
	// 1. fetch metadata from authorization url
	// 2. if fails — throw error
	//
	// artifacts on this step:
	// 1. path to registering client
	// 2. authorization endpoint
	// 3. token endpoint
	//
	// If we can't find registration endpoint — throw an error with resource documentation url.

	var (
		registrationEndpoint  *url.URL
		authorizationEndpoint *url.URL
		tokenEndpoint         *url.URL
		authScopes            []string
	)

	var authMetadataErr error
	for _, authServer := range authServers {
		var metadataResp *oauth.GetAuthServerMetadataResponse
		metadataResp, authMetadataErr = client.GetAuthServerMetadataWithResponse(ctx, func(ctx context.Context, req *http.Request) error {
			cloned := *authServer
			if !strings.HasPrefix(cloned.Path, "/.well-known/oauth-authorization-server") {
				cloned.Path = path.Join("/.well-known/oauth-authorization-server", cloned.Path)
			}
			req.URL = &cloned
			req.Host = cloned.Host
			return nil
		})

		if authMetadataErr != nil {
			continue // try next
		}

		if metadataResp.StatusCode() != http.StatusOK {
			authMetadataErr = fmt.Errorf("unexpected status code %d when requesting %s", metadataResp.StatusCode(), authServer.String())
			continue
		}

		m := metadataResp.JSONDefault
		if m == nil {
			authMetadataErr = fmt.Errorf("empty metadata response")
			continue
		}

		if m.RegistrationEndpoint != nil {
			registrationEndpoint, err = url.Parse(*m.RegistrationEndpoint)
			if err != nil {
				authMetadataErr = fmt.Errorf("failed to parse registration endpoint: %w", err)
				continue
			}
		} else {
			authMetadataErr = oauthhandler.ErrDynamicClientRegistrationNotSupported(resourceDocumentationURL)
			continue
		}

		if m.AuthorizationEndpoint != nil {
			authorizationEndpoint, err = url.Parse(*m.AuthorizationEndpoint)
			if err != nil {
				authMetadataErr = fmt.Errorf("failed to parse authorization endpoint: %w", err)
				continue
			}
		}
		if m.TokenEndpoint != nil {
			tokenEndpoint, err = url.Parse(*m.TokenEndpoint)
			if err != nil {
				authMetadataErr = fmt.Errorf("failed to parse token endpoint: %w", err)
				continue
			}
		}
		if m.ScopesSupported != nil {
			authScopes = *m.ScopesSupported
		}

		authMetadataErr = nil
		break
	}

	if authMetadataErr != nil {
		return nil, time.Time{}, fmt.Errorf("failed to get authorization server metadata: %w", authMetadataErr)
	}

	// Step 3: registering client.
	//
	// 1. posting to registration endpoint
	// 2. if fails — throw error
	//
	// artifacts on this step:
	// 1. client id
	// 2. client secret
	// 3. expires at

	grantTypes := []string{"authorization_code", "refresh_token"}
	responseTypes := []string{"code"}
	tokenEndpointAuthMethod := "client_secret_basic"

	finalScopes := h.defaultScopes
	if len(resourceScopes) > 0 {
		finalScopes = resourceScopes
	} else if len(authScopes) > 0 {
		finalScopes = authScopes
	}

	scope := strings.Join(finalScopes, " ")

	registerResp, err := client.RegisterOauthClientWithResponse(ctx, "", oauth.RegisterOauthClientJSONRequestBody{
		ClientName:              clientName,
		RedirectUris:            []string{redirect.String()},
		TokenEndpointAuthMethod: &tokenEndpointAuthMethod,
		GrantTypes:              &grantTypes,
		ResponseTypes:           &responseTypes,
		Scope:                   &scope,
	}, useExactURI(registrationEndpoint))
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to register client: %w", err)
	}

	if registerResp.StatusCode() != http.StatusCreated && registerResp.StatusCode() != http.StatusOK {
		errStr := fmt.Sprintf("unexpected status code %d when registering client at %s", registerResp.StatusCode(), registrationEndpoint)
		if len(registerResp.Body) > 0 {
			errStr += fmt.Sprintf(": %s", string(registerResp.Body))
		}
		return nil, time.Time{}, errors.New(errStr)
	}

	clientData := registerResp.JSONDefault
	if clientData == nil {
		return nil, time.Time{}, fmt.Errorf("empty response when registering client")
	}

	if clientData.ClientSecretExpiresAt != nil && *clientData.ClientSecretExpiresAt > 0 {
		expiresAt = time.Unix(int64(*clientData.ClientSecretExpiresAt), 0)
	}

	var clientSecret string
	if clientData.ClientSecret != nil {
		clientSecret = *clientData.ClientSecret
	}

	var authURL, tokenURL string
	if authorizationEndpoint != nil {
		authURL = authorizationEndpoint.String()
	}
	if tokenEndpoint != nil {
		tokenURL = tokenEndpoint.String()
	}

	return &oauth2.Config{
		ClientID:     clientData.ClientId,
		ClientSecret: clientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:       authURL,
			DeviceAuthURL: "",
			TokenURL:      tokenURL,
			AuthStyle:     oauth2.AuthStyleAutoDetect,
		},
		RedirectURL: redirect.String(),
		Scopes:      finalScopes,
	}, expiresAt, nil
}

// TODO: immideately return error, if we stuck with infrastructure error, and not "not found" or "forbidden"
func discoverProtectedResourceMetadata(ctx context.Context, client oauth.ClientWithResponsesInterface, originURL, suggestedURL *url.URL) (*oauth.ProtectedResourceMetadata, error) {
	if suggestedURL != nil {
		resp, err := client.GetResourceMetadataWithResponse(ctx, useExactURI(suggestedURL))
		if err == nil && resp.StatusCode() < http.StatusBadRequest {
			if resp.JSONDefault == nil {
				return nil, fmt.Errorf("empty response when discovering protected resource metadata at %q", resp.HTTPResponse.Request.URL.String())
			}
			return resp.JSONDefault, nil
		}
	}

	if originURL == nil {
		return nil, fmt.Errorf("origin URL is required to discover metadata")
	}

	resp, err := client.GetResourceMetadataWithResponse(ctx, useGranularProtectedResourceEndpoint(originURL))
	if err == nil && resp.StatusCode() < http.StatusBadRequest {
		if resp.JSONDefault == nil {
			return nil, fmt.Errorf("empty response when discovering protected resource metadata at %q", resp.HTTPResponse.Request.URL.String())
		}
		return resp.JSONDefault, nil
	}

	resp, err = client.GetResourceMetadataWithResponse(ctx, useDomainProtectedResourceEndpoint(originURL))
	if err == nil && resp.StatusCode() < http.StatusBadRequest {
		if resp.JSONDefault == nil {
			return nil, fmt.Errorf("empty response when discovering protected resource metadata at %q", resp.HTTPResponse.Request.URL.String())
		}
		return resp.JSONDefault, nil
	}

	return nil, fmt.Errorf("failed to discover protected resource metadata: %w", err)
}

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

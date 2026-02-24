package oauth

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
)

func (h *Handler) RegisterClient(ctx context.Context, u *url.URL, clientName string, redirect *url.URL, opts ...ports.RegisterClientOption) (cfg *oauth2.Config, expiresAt time.Time, err error) {
	p := ports.RegisterClientParams(opts...)

	metadata, err := h.getServerMetadata(ctx, u)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to get server metadata: %w", err)
	}

	if metadata.RegistrationEndpoint == "" {
		return nil, time.Time{}, errors.New("server does not support dynamic client registration")
	}

	registrationEndpoint, err := url.Parse(metadata.RegistrationEndpoint)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to parse registration endpoint: %w", err)
	}

	resp, err := fetchRegisterClient(ctx, h.client, registrationEndpoint, &registerClientRequestBody{
		ClientName:              clientName,
		RedirectURIs:            []string{redirect.String()},
		TokenEndpointAuthMethod: "client_secret_basic",
		GrantTypes:              []string{"authorization_code", "refresh_token"},
		ResponseTypes:           []string{"code"},
		Scope:                   strings.Join(h.authScopes, " "),
	})
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("registering client: %w", err)
	}

	if resp.ExpiresAt > 0 {
		expiresAt = time.Unix(resp.ExpiresAt, 0)
	}

	return &oauth2.Config{
		ClientID:     resp.ClientID,
		ClientSecret: resp.ClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:       metadata.AuthorizationEndpoint,
			DeviceAuthURL: "",
			TokenURL:      metadata.TokenEndpoint,
			AuthStyle:     oauth2.AuthStyleAutoDetect,
		},
		RedirectURL: redirect.String(),
		Scopes:      h.authScopes,
	}, expiresAt, nil
}

func (h *Handler) getServerMetadata(ctx context.Context, u *url.URL) (*serverMetadataResponse, error) {
	// Try to discover the authorization server via OAuth Protected Resource
	// as per RFC 9728 (https://datatracker.ietf.org/doc/html/rfc9728)

	// Try to fetch the OAuth Protected Resource metadata
	protectedResourceURL := &url.URL{
		Scheme: u.Scheme,
		Host:   u.Host,
		Path:   "/.well-known/oauth-protected-resource",
	}

	authServerScheme, authServerHost := u.Scheme, u.Host

	protectedResource, err := fetchProtectedResource(ctx, h.client, protectedResourceURL, nil)
	if err == nil && len(protectedResource.AuthorizationServers) > 0 {
		authServer, err := url.Parse(protectedResource.AuthorizationServers[0])
		if err != nil {
			return nil, fmt.Errorf("received invalid authorization server URL %q: %w", protectedResource.AuthorizationServers[0], err)
		}

		authServerScheme, authServerHost = authServer.Scheme, authServer.Host
	} else if errors.Is(err, errNotFound) {
		// using default endpoint then
	} else if err != nil {
		return nil, fmt.Errorf("failed to fetch protected resource metadata: %w", err)
	}

	// Try OpenID Connect discovery first
	metadata, err := fetchMetadataFromURL(ctx, h.client, &url.URL{
		Scheme: authServerScheme,
		Host:   authServerHost,
		Path:   "/.well-known/openid-configuration",
	}, nil)
	if err == nil {
		return metadata, nil
	}

	// If OpenID Connect discovery fails, try OAuth Authorization Server Metadata
	metadata, err = fetchMetadataFromURL(ctx, h.client, &url.URL{
		Scheme: authServerScheme,
		Host:   authServerHost,
		Path:   "/.well-known/oauth-authorization-server",
	}, nil)
	if err == nil {
		return metadata, nil
	}

	// If both discovery methods fail, use default endpoints based on the authorization server URL
	metadata, err = getDefaultEndpoints(&url.URL{Scheme: authServerScheme, Host: authServerHost})
	if err != nil {
		return nil, fmt.Errorf("failed to get default endpoints: %w", err)
	}

	return metadata, nil
}

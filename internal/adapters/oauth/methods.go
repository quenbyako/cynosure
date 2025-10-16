package oauth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
)

type serverMetadataResponse struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	RegistrationEndpoint              string   `json:"registration_endpoint,omitempty"`
	JwksURI                           string   `json:"jwks_uri,omitempty"`
	ScopesSupported                   []string `json:"scopes_supported,omitempty"`
	ResponseTypesSupported            []string `json:"response_types_supported"`
	GrantTypesSupported               []string `json:"grant_types_supported,omitempty"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported,omitempty"`
}

// fetchMetadataFromURL fetches and parses OAuth server metadata from a URL
func fetchMetadataFromURL(ctx context.Context, client *http.Client, metadataURL *url.URL, additionalHeaders http.Header) (*serverMetadataResponse, error) {
	req, err := http.NewRequest(http.MethodGet, metadataURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating metadata request: %w", err)
	}

	req.Header.Set(HeaderAccept, mimeJSON)
	for key, values := range additionalHeaders {
		req.Header[key] = slices.Clone(values)
	}

	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("sending metadata request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// If metadata discovery fails, don't set any metadata
		return nil, fmt.Errorf("fetching metadata from %s: %s", metadataURL.String(), resp.Status)
	}

	var metadata serverMetadataResponse
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("decoding metadata response: %w", err)
	}

	return &metadata, nil
}

// OAuthProtectedResource represents the response from /.well-known/oauth-protected-resource
type OAuthProtectedResource struct {
	AuthorizationServers []string `json:"authorization_servers"` // url
	Resource             string   `json:"resource"`
	ResourceName         string   `json:"resource_name,omitempty"`
}

func fetchProtectedResource(ctx context.Context, client *http.Client, u *url.URL, additionalHeaders http.Header) (*OAuthProtectedResource, error) {
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating protected resource request: %w", err)
	}

	req.Header.Set(HeaderAccept, mimeJSON)
	for key, values := range additionalHeaders {
		req.Header[key] = slices.Clone(values)
	}

	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("sending protected resource request: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var protectedResource OAuthProtectedResource
		if err := json.NewDecoder(resp.Body).Decode(&protectedResource); err != nil {
			return nil, fmt.Errorf("decoding response: %w", err)
		}

		return &protectedResource, nil

	case http.StatusNotFound:
		return nil, errNotFound

	default:
		return nil, fmt.Errorf("fetching protected resource metadata: %s", resp.Status)
	}

}

type registerClientRequestBody struct {
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	Scope                   string   `json:"scope"`
}

type registerClientResponse struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret,omitempty"`
	ExpiresAt    int64  `json:"client_secret_expires_at,omitempty"`
}

func fetchRegisterClient(ctx context.Context, client *http.Client, u *url.URL, body *registerClientRequestBody) (*registerClientResponse, error) {
	reqBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshaling registration request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, u.String(), bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("creating registration request: %w", err)
	}

	req.Header.Set(HeaderContentType, mimeJSON)
	req.Header.Set(HeaderAccept, mimeJSON)

	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to send registration request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, extractOAuthError(body, resp.StatusCode, "registration request failed")
	}

	var regResponse registerClientResponse
	if err := json.NewDecoder(resp.Body).Decode(&regResponse); err != nil {
		return nil, fmt.Errorf("failed to decode registration response: %w", err)
	}

	return &regResponse, nil
}

package oauth

import (
	"context"
	"net/http"

	"golang.org/x/oauth2"
)

// TODO(refactor): This is a duplicate of internal/adapters/ory/oauth_client_wrapper.go
// It should be moved to a shared package (e.g. contrib/oauth2ext) once it's used in 3+ places.

// injectedConfig wraps [oauth2.Config] and [http.Client]. It ensures that any
// token operations (exchange or refresh) use our explicit client, without
// leaking the context hack into the calling business logic.
type injectedConfig struct {
	*oauth2.Config
	client *http.Client
}

// Exchange performs token exchange while explicitly injecting the client into
// the library's context.
func (c *injectedConfig) Exchange(
	ctx context.Context,
	code string,
	opts ...oauth2.AuthCodeOption,
) (*oauth2.Token, error) {
	if c.client != nil {
		ctx = context.WithValue(ctx, oauth2.HTTPClient, c.client)
	}

	//nolint:wrapcheck // must not modify wrapped object behavior
	return c.Config.Exchange(ctx, code, opts...)
}

// TokenSource returns a token source that will use our client for auto-refresh.
// The context (and thus the client) is captured by the library at this point.
func (c *injectedConfig) TokenSource(ctx context.Context, t *oauth2.Token) oauth2.TokenSource {
	if c.client != nil {
		ctx = context.WithValue(ctx, oauth2.HTTPClient, c.client)
	}

	return c.Config.TokenSource(ctx, t)
}

// injectTransport creates a wrapper around the config bound to a specific HTTP
// client.
func injectTransport(client *http.Client, config *oauth2.Config) *injectedConfig {
	if config == nil {
		return nil
	}

	return &injectedConfig{
		Config: config,
		client: client,
	}
}

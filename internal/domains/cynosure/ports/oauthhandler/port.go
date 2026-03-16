// Package oauthhandler defines OAuth handler port.
package oauthhandler

import (
	"context"
	"net/url"
	"time"

	"golang.org/x/oauth2"
)

// Port manages OAuth 2.0 flows for MCP server authentication.
// Supports dynamic client registration, token exchange, and refresh operations.
type Port interface {
	// RegisterClient performs dynamic client registration (RFC 7591) if server
	// supports it. Returns OAuth config and optional client credentials
	// expiration.
	//
	// Options:
	//
	//  - [WithSuggestedLink] — suggests a link to use for registration.
	//
	// See next test suites to find how it works:
	//
	//  - [TestRegisterClient] — dynamic client registration with various
	//     servers
	//
	// Throws:
	//
	//  - [ErrAuthUnsupported] if auth is not supported, server may just connect
	//    without auth.
	//  - [ErrServerUnreachable] if registration endpoint is unavailable.
	//  - [DynamicClientRegistrationNotSupportedError] if server does not support
	//    dynamic client registration.
	RegisterClient(
		ctx context.Context, resourceURL *url.URL, clientName string, setRedirect *url.URL,
		opts ...RegisterClientOption,
	) (cfg *oauth2.Config, expiresAt time.Time, err error)

	// RefreshToken obtains a new access token using refresh token. Implements
	// standard OAuth 2.0 refresh flow.
	//
	// See next test suites to find how it works:
	//
	//  - [TestRefreshToken] — refreshing OAuth tokens
	//
	// Throws:
	//
	//  - [ErrInvalidCredentials] if refresh token is invalid or expired.
	RefreshToken(
		ctx context.Context, config *oauth2.Config, token *oauth2.Token,
	) (*oauth2.Token, error)

	// Exchange exchanges authorization code for access token. Supports PKCE
	// flow via verifier parameter. Standard OAuth 2.0 authorization code flow.
	//
	// See next test suites to find how it works:
	//
	//  - [TestExchange] — exchanging authorization code with PKCE support
	//
	// Throws:
	//
	//  - [ErrInvalidCredentials] if authorization code is invalid.
	Exchange(
		ctx context.Context, config *oauth2.Config, code string, verifier []byte,
	) (*oauth2.Token, error)
}

func defaultRegisterClientParams() *registerClientParams {
	return &registerClientParams{
		suggestedProtectedResource: nil,
	}
}

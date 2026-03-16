// Package mcpmock provides parameters for the mock MCP server.
package mcpmock

// Parameters for EACH test:
//
// 1. transport:
// 	  - SSE transport
//    - HTTP transport
// 2. auth:
//    - no auth
//    - returns Www-Authorize header
//    - doesn't return anything
//    - allows to connect BOTH authorized and unauthorized
// 3. process of oauth discovery:
//    - doesn't have any well known endpoint
//    - has only authorize flow parameters, but not client registration
//    - has both authorize flow and client registration parameters
// 4. process of registry oauth client:
//    - supports registration
//    - does not support registration
// 4. process of oauth flow:
//    - throws error on auth link
//    - returns url for authorize
//    - instantly returns token
// 5. request with oauth token:
//    - valid token
//    - expired token, but valid refresh token
//    - expired token, expired refresh token

//go:generate go tool stringer -type=TransportType,AuthRequirement,MetadataDiscovery,OAuthDiscovery,OAuthRegistration,OAuthFlow,OAuthTokenStatus,AuthType -linecomment -output=parameters_string.gen.go

// TransportType defines the protocol used by the server.
type TransportType uint8

const (
	// SSE transport
	TransportSSE TransportType = iota // sse
	// HTTP transport
	TransportHTTP // http
)

// AuthRequirement defines how the server responds to unauthenticated requests.
type AuthRequirement uint8

const (
	// No auth
	AuthNone AuthRequirement = iota // none
	// Allows to connect BOTH authorized and unauthorized
	AuthOptional // optional
	// Returns Www-Authenticate header
	AuthRequired // required
	// Required auth, but Doesn't return anything (just 401)
	AuthNoHeader // no_header
)

type MetadataDiscovery uint8

const (
	// doesn't have any .well-known endpoint
	MetadataDiscoveryNone MetadataDiscovery = iota // none
	// https://example.com/.well-known/oauth-protected-resource
	MetadataDiscoveryRoot // root
	// for https://example.com/public/mcp it will be
	// https://example.com/.well-known/oauth-protected-resource/public/mcp
	MetadataDiscoveryPathSpecific // path_specific
	// writes into WWW-Authenticate header
	MetadataDiscoveryPathExplicit // path_explicit
)

// OAuthDiscovery defines the OAuth discovery capabilities of the server.
type OAuthDiscovery uint8

const (
	// Doesn't have any well known endpoint
	OAuthDiscoveryNone OAuthDiscovery = iota // none
	// Has only authorize flow parameters, but not client registration
	OAuthDiscoveryAuthOnly // auth_only
	// Has both authorize flow and client registration parameters
	OAuthDiscoveryFull // full
)

// OAuthRegistration defines if the server supports dynamic client registration.
type OAuthRegistration uint8

const (
	// Not supported
	OAuthRegistrationNotSupported OAuthRegistration = iota // not_supported
	// Supported
	OAuthRegistrationSupported // supported
)

// OAuthFlow defines the behavior during the OAuth flow.
type OAuthFlow uint8

const (
	// Returns url for authorize
	OAuthFlowReturnsURL OAuthFlow = iota // returns_url
	// Throws error on auth link
	OAuthFlowAuthLinkError // auth_link_error
	// Instantly returns token
	OAuthFlowInstantlyReturnsToken // instantly_returns_token
)

// OAuthTokenStatus defines the simulated state of the token provided to the server.
type OAuthTokenStatus uint8

const (
	// both token and refresh token are valid
	TokenValid OAuthTokenStatus = iota // valid
	// expired token, but valid refresh token
	TokenExpiredRefreshValid // expired_refresh_valid
	// expired token, expired refresh token
	TokenExpiredRefreshExpired // expired_refresh_expired
)

type AuthType uint8

const (
	// No token_type returned in JSON, required to check that it's "Bearer"
	AuthTypeNone AuthType = iota // none
	// Standard "Bearer", but explicitly mentioned in discovery
	AuthTypeBearer // bearer
	// Totally random string like "X-Token-..."
	AuthTypeRandomized // randomized
)

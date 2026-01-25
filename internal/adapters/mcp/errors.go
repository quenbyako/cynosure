package mcp

import (
	"errors"
	"fmt"
)

// ErrNotMCPServer is returned when both Streamable and SSE protocols fail,
// indicating the address is not an MCP server.
var ErrNotMCPServer = errors.New("address is not an MCP server")

// TransportError categorizes errors from MCP transport attempts.
// It distinguishes between infrastructure failures (fail immediately),
// protocol mismatches (trigger fallback), and auth errors (update token).
type TransportError interface {
	error
	// Unwrap returns the underlying error
	Unwrap() error
}

// InfrastructureError represents network/infrastructure failures that should
// fail immediately without attempting protocol fallback.
// Examples: DNS failures, connection refused, TLS errors
type InfrastructureError struct {
	cause error
}

func ErrInfrastructure(err error) error {
	return &InfrastructureError{cause: err}
}

func (e *InfrastructureError) Error() string {
	return fmt.Sprintf("infrastructure error: %v", e.cause)
}

func (e *InfrastructureError) Unwrap() error {
	return e.cause
}

// ProtocolError represents protocol-level mismatches that should trigger
// fallback to an alternative protocol.
// Examples: HTTP 400/404/405, unexpected EOF, unknown response format
type ProtocolError struct {
	cause error
}

func ErrProtocol(err error) error {
	return &ProtocolError{cause: err}
}

func (e *ProtocolError) Error() string {
	return fmt.Sprintf("protocol error: %v", e.cause)
}

func (e *ProtocolError) Unwrap() error {
	return e.cause
}

// AuthError represents authentication/authorization failures.
// Examples: HTTP 401/403, expired token
type AuthError struct {
	cause error
}

func ErrAuth(err error) error {
	return &AuthError{cause: err}
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("auth error: %v", e.cause)
}

func (e *AuthError) Unwrap() error {
	return e.cause
}

// HTTPStatusError represents an HTTP error response.
// This type is used internally for classification purposes.
type HTTPStatusError struct {
	StatusCode int
	Status     string
	URL        interface{} // Can be *url.URL or string
}

func (e *HTTPStatusError) Error() string {
	if e.URL != nil {
		return fmt.Sprintf("HTTP %d: %s for %v", e.StatusCode, e.Status, e.URL)
	}
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Status)
}

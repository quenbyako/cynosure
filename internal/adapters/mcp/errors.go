package mcp

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/quenbyako/cynosure/internal/adapters/mcp/rfc9110"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
)

// ErrNotMCPServer is returned when both Streamable and SSE protocols fail,
// indicating the address is not an MCP server.
var ErrNotMCPServer = errors.New("address is not an MCP server")

// TransportError categorizes errors from MCP transport attempts.
type TransportError interface {
	error
	Unwrap() error
}

// InfrastructureError represents network/infrastructure failures.
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

// ProtocolError represents protocol-level mismatches.
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

// HTTPStatusError represents an HTTP error response.
// This type is used internally for classification purposes.
type HTTPStatusError struct {
	StatusCode     int
	Status         string
	URL            string
	ResponseHeader http.Header
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("HTTP %d: %s for %s", e.StatusCode, e.Status, e.URL)
}

// NOTE: for some MCP servers there was "scopes" and "authorization_uri"
// metadata keys in WWW-Authenticate header. However, there is no standard for
// those keys. Since both of them look very optional — we just ignoring them for
// now.
func extractAuthError(resp *http.Response) *ports.RequiresAuthError {
	if resp.Header == nil {
		return ports.ErrRequiresAuth(nil)
	}
	wwwAuth := resp.Header.Get("WWW-Authenticate")
	if wwwAuth == "" {
		return ports.ErrRequiresAuth(nil) // No metadata suggested
	}

	challenges, ok := rfc9110.ParseWWWAuthenticate(wwwAuth)
	if !ok {
		// fallback: some MCP servers returns just an URL link to this header.
		// If it's a valid URL, we can use it
		if metadataURL, err := url.Parse(wwwAuth); err == nil && metadataURL.IsAbs() {
			return ports.ErrRequiresAuth(metadataURL)
		}

		return ports.ErrRequiresAuth(nil)
	}

	for _, ch := range challenges {
		metadataURLStr, ok := ch.Params["resource_metadata"]

		if !ok {
			// NON-STANDARD: some servers are using "resource" instead of
			// "resource_metadata"
			metadataURLStr, ok = ch.Params["resource"]
		}

		if ok {
			metadataURL, err := url.Parse(metadataURLStr)
			if err != nil {
				continue
			}

			// Resolve relative URL against original request URL
			if !metadataURL.IsAbs() {
				baseURL, err := url.Parse(resp.Request.URL.String())
				if err == nil {
					metadataURL = baseURL.ResolveReference(metadataURL)
				}
			}

			return ports.ErrRequiresAuth(metadataURL)
		}
	}

	return ports.ErrRequiresAuth(nil)
}

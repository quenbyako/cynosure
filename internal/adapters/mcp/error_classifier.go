package mcp

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
)

// MapError maps internal transport errors to domain port errors.
func MapError(err error) error {
	if err == nil {
		return nil
	}

	classified := ClassifyTransportError(err)

	if e := new(AuthError); errors.As(classified, &e) {
		return fmt.Errorf("%w: %v", ports.ErrAuthRequired, e.cause)
	}

	if e := new(InfrastructureError); errors.As(classified, &e) {
		return fmt.Errorf("%w: %v", ports.ErrServerUnreachable, e.cause)
	}

	if e := new(ProtocolError); errors.As(classified, &e) {
		return fmt.Errorf("%w: %v", ports.ErrProtocolNotSupported, e.cause)
	}

	if errors.Is(err, ErrNotMCPServer) {
		return fmt.Errorf("%w: %v", ports.ErrServerUnreachable, err)
	}

	return err
}

// ClassifyTransportError categorizes an error into InfrastructureError, ProtocolError, or AuthError.
// This is used to determine whether to attempt protocol fallback or fail immediately.
//
// Returns the original error wrapped in the appropriate classification type:
//   - InfrastructureError: Network/DNS/TLS failures (no fallback, fail immediately)
//   - ProtocolError: HTTP 4xx client errors, malformed responses (trigger fallback)
//   - AuthError: HTTP 401/403, token errors (update token)
func ClassifyTransportError(err error) error {
	if err == nil {
		return nil
	}

	// Check for HTTP status errors first
	if httpErr := new(HTTPStatusError); errors.As(err, &httpErr) {
		return classifyHTTPError(httpErr.StatusCode, err)
	}

	// Check for network errors
	if netOpErr := new(net.OpError); errors.As(err, &netOpErr) {
		return classifyNetworkError(netOpErr, err)
	}

	// Check for DNS errors
	if dnsErr := new(net.DNSError); errors.As(err, &dnsErr) {
		return &InfrastructureError{cause: err}
	}

	// Check error message patterns
	errMsg := strings.ToLower(err.Error())

	// TLS errors are infrastructure
	if strings.Contains(errMsg, "tls") || strings.Contains(errMsg, "certificate") {
		return &InfrastructureError{cause: err}
	}

	// Auth-related errors
	if strings.Contains(errMsg, "token expired") || strings.Contains(errMsg, "unauthorized") {
		return &AuthError{cause: err}
	}

	// Protocol-related errors (including MCP handshake failures)
	if strings.Contains(errMsg, "unexpected eof") ||
		strings.Contains(errMsg, "unknown response") ||
		strings.Contains(errMsg, "invalid response") ||
		strings.Contains(errMsg, "malformed") ||
		strings.Contains(errMsg, "bad request") ||
		strings.Contains(errMsg, "not found") ||
		strings.Contains(errMsg, "session not found") ||
		strings.Contains(errMsg, "initialize") {
		return &ProtocolError{cause: err}
	}

	// Default: treat as infrastructure error (fail fast)
	return &InfrastructureError{cause: err}
}

// classifyHTTPError classifies HTTP status codes into error categories
func classifyHTTPError(statusCode int, err error) error {
	switch {
	case statusCode == 401 || statusCode == 403:
		// Authentication/authorization errors
		return &AuthError{cause: err}
	case statusCode == 400 || statusCode == 404 || statusCode == 405:
		// Protocol mismatch errors (client-side, likely wrong protocol)
		return &ProtocolError{cause: err}
	case statusCode >= 500:
		// Server errors - fail fast, don't fallback
		return &InfrastructureError{cause: err}
	default:
		// Other client errors - treat as protocol issues
		return &ProtocolError{cause: err}
	}
}

// classifyNetworkError classifies network operation errors
func classifyNetworkError(netErr *net.OpError, err error) error {
	// Check for specific syscall errors
	if netErr.Err != nil {
		// Connection refused, network unreachable, etc.
		if errors.Is(netErr.Err, syscall.ECONNREFUSED) ||
			errors.Is(netErr.Err, syscall.ENETUNREACH) ||
			errors.Is(netErr.Err, syscall.EHOSTUNREACH) {
			return &InfrastructureError{cause: err}
		}

		// Check if it's a timeout error (temporary, not infrastructure)
		if netErr.Timeout() {
			// Timeouts might be retryable, treat as infrastructure for now
			return &InfrastructureError{cause: err}
		}
	}

	// Default network errors to infrastructure
	return &InfrastructureError{cause: err}
}

// SynthesizeNotMCPServerError creates a comprehensive error when both protocols fail.
// It indicates that the address is likely not an MCP server.
func SynthesizeNotMCPServerError(url string, streamableErr, sseErr error) error {
	return fmt.Errorf("%w (both protocols failed for %s): streamable=%v, sse=%v",
		ErrNotMCPServer, url, streamableErr, sseErr)
}

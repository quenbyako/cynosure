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

	// If it's already a domain error or auth error, don't re-map it
	if errors.As(err, new(*ports.RequiresAuthError)) {
		return err
	}
	if errors.Is(err, ports.ErrServerUnreachable) ||
		errors.Is(err, ports.ErrProtocolNotSupported) ||
		errors.Is(err, ports.ErrInvalidCredentials) {
		return err
	}

	classified := ClassifyTransportError(err)

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

// ClassifyTransportError categorizes an error into InfrastructureError or ProtocolError.
func ClassifyTransportError(err error) error {
	if err == nil {
		return nil
	}

	// Check for HTTP status errors first
	if httpErr := new(HTTPStatusError); errors.As(err, &httpErr) {
		return classifyHTTPError(httpErr.StatusCode, httpErr)
	}

	// Check for network errors
	if netOpErr := new(net.OpError); errors.As(err, &netOpErr) {
		return classifyNetworkError(netOpErr, netOpErr)
	}

	// Check for DNS errors
	if dnsErr := new(net.DNSError); errors.As(err, &dnsErr) {
		return &InfrastructureError{cause: dnsErr}
	}

	// Check error message patterns
	errMsg := strings.ToLower(err.Error())

	// TLS errors are infrastructure
	if strings.Contains(errMsg, "tls") || strings.Contains(errMsg, "certificate") {
		return &InfrastructureError{cause: err}
	}

	// Protocol-related errors
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
		// Auth errors don't trigger fallback, they need special handling in
		// MapError
		return err
	case statusCode == 400 || statusCode == 404 || statusCode == 405:
		return &ProtocolError{cause: err}
	case statusCode >= 500:
		return &InfrastructureError{cause: err}
	default:
		return &ProtocolError{cause: err}
	}
}

// classifyNetworkError classifies network operation errors
func classifyNetworkError(netErr *net.OpError, err error) error {
	if netErr.Err != nil {
		if errors.Is(netErr.Err, syscall.ECONNREFUSED) ||
			errors.Is(netErr.Err, syscall.ENETUNREACH) ||
			errors.Is(netErr.Err, syscall.EHOSTUNREACH) {
			return &InfrastructureError{cause: err}
		}
		if netErr.Timeout() {
			return &InfrastructureError{cause: err}
		}
	}
	return &InfrastructureError{cause: err}
}

// SynthesizeNotMCPServerError creates a comprehensive error when both protocols fail.
func SynthesizeNotMCPServerError(url string, streamableErr, sseErr error) error {
	return fmt.Errorf("%w (both protocols failed for %s): streamable=%v, sse=%v",
		ErrNotMCPServer, url, streamableErr, sseErr)
}

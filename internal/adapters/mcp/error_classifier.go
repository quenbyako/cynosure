package mcp

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"syscall"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/toolclient"
)

// MapError maps internal transport errors to domain port errors.
func MapError(err error) error {
	if err == nil {
		return nil
	}

	if mapped, ok := tryMapKnownError(err); ok {
		return mapped
	}

	classified := ClassifyTransportError(err)

	var infraErr *InfrastructureError
	if errors.As(classified, &infraErr) {
		return fmt.Errorf("%w: %w", toolclient.ErrServerUnreachable, infraErr.cause)
	}

	var protoErr *ProtocolError
	if errors.As(classified, &protoErr) {
		return fmt.Errorf("%w: %w", toolclient.ErrProtocolNotSupported, protoErr.cause)
	}

	if errors.Is(err, ErrNotMCPServer) {
		return fmt.Errorf("%w: %w", toolclient.ErrServerUnreachable, err)
	}

	return err
}

func tryMapKnownError(err error) (error, bool) {
	if errors.Is(err, toolclient.ErrServerUnreachable) ||
		errors.Is(err, toolclient.ErrProtocolNotSupported) ||
		errors.Is(err, toolclient.ErrInvalidCredentials) {
		return err, true
	}

	if e := new(toolclient.RequiresAuthError); errors.As(err, &e) {
		return err, true
	}

	if e := new(ports.RequiresAuthError); errors.As(err, &e) {
		return toolclient.ErrRequiresAuth(e.Endpoint()), true
	}

	if errors.Is(err, ports.ErrServerUnreachable) {
		return toolclient.ErrServerUnreachable, true
	}

	if errors.Is(err, ports.ErrProtocolNotSupported) {
		return toolclient.ErrProtocolNotSupported, true
	}

	if errors.Is(err, ports.ErrInvalidCredentials) {
		return toolclient.ErrInvalidCredentials, true
	}

	return nil, false
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
	return classifyErrorByMessage(err)
}

func classifyErrorByMessage(err error) error {
	errMsg := strings.ToLower(err.Error())

	// TLS errors are infrastructure
	if strings.Contains(errMsg, "tls") || strings.Contains(errMsg, "certificate") {
		return &InfrastructureError{cause: err}
	}

	// Protocol-related errors
	if isProtocolErrorMessage(errMsg) {
		return &ProtocolError{cause: err}
	}

	// Default: treat as infrastructure error (fail fast)
	return &InfrastructureError{cause: err}
}

func isProtocolErrorMessage(errMsg string) bool {
	patterns := []string{
		"unexpected eof",
		"unknown response",
		"invalid response",
		"malformed",
		"bad request",
		"not found",
		"session not found",
		"initialize",
	}
	for _, p := range patterns {
		if strings.Contains(errMsg, p) {
			return true
		}
	}

	return false
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
	case statusCode >= http.StatusInternalServerError:
		return &InfrastructureError{cause: err}
	default:
		return &ProtocolError{cause: err}
	}
}

// classifyNetworkError classifies network operation errors
func classifyNetworkError(netErr *net.OpError, err error) error {
	if netErr.Err == nil {
		return &InfrastructureError{cause: err}
	}

	if errors.Is(netErr.Err, syscall.ECONNREFUSED) ||
		errors.Is(netErr.Err, syscall.ENETUNREACH) ||
		errors.Is(netErr.Err, syscall.EHOSTUNREACH) {
		return &InfrastructureError{cause: err}
	}

	if netErr.Timeout() {
		return &InfrastructureError{cause: err}
	}

	return &InfrastructureError{cause: err}
}

// SynthesizeNotMCPServerError creates a comprehensive error when both protocols fail.
func SynthesizeNotMCPServerError(targetURL string, streamableErr, sseErr error) error {
	return fmt.Errorf("%w (both protocols failed for %s): streamable=%w, sse=%w",
		ErrNotMCPServer, targetURL, streamableErr, sseErr)
}

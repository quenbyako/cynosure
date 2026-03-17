package mcp_test

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"syscall"
	"testing"

	. "github.com/quenbyako/cynosure/internal/adapters/mcp"
)

// Test error classification for infrastructure errors
func TestClassifyError_Infrastructure(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		err  error
		want bool
	}{{
		name: "DNS lookup failure",
		err:  noSuchHostErr("nonexistent.example.com"),
		want: true,
	}, {
		name: "Connection refused",
		err:  netOpError("dial", "tcp", syscall.ECONNREFUSED),
		want: true,
	}, {
		name: "Network unreachable",
		err:  netOpError("dial", "tcp", syscall.ENETUNREACH),
		want: true,
	}, {
		name: "TLS handshake failure",
		err:  errors.New("tls: handshake failure"),
		want: true,
	}, {
		name: "Timeout error",
		err:  netOpError("read", "tcp", &timeoutError{}),
		want: true, // Timeouts are infrastructure failures (not protocol mismatches)
	}} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			classified := ClassifyTransportError(tt.err)
			if e := new(InfrastructureError); errors.As(classified, &e) != tt.want {
				t.Errorf(
					"classifyError(%v).IsInfrastructure() = %v, want %v",
					tt.err, !tt.want, tt.want,
				)
			}
		})
	}
}

// Test error classification for protocol errors
func TestClassifyError_Protocol(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		err  error
		want bool
	}{{
		name: "HTTP 400 Bad Request",
		err:  newStatusError(http.StatusBadRequest, "400 Bad Request"),
		want: true,
	}, {
		name: "HTTP 404 Not Found",
		err:  newStatusError(http.StatusNotFound, "404 Not Found"),
		want: true,
	}, {
		name: "HTTP 405 Method Not Allowed",
		err:  newStatusError(http.StatusMethodNotAllowed, "405 Method Not Allowed"),
		want: true,
	}, {
		name: "Unexpected EOF",
		err:  errors.New("unexpected EOF"),
		want: true,
	}, {
		name: "Invalid response format",
		err:  errors.New("unknown response type"),
		want: true,
	}, {
		name: "HTTP 500 Server Error",
		err:  newStatusError(http.StatusInternalServerError, "500 Internal Server Error"),
		want: false, // 5xx are server errors, not protocol mismatches
	}} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			classified := ClassifyTransportError(tt.err)
			if e := new(ProtocolError); errors.As(classified, &e) != tt.want {
				t.Errorf("classifyError(%v).IsProtocol() = %v, want %v", tt.err, !tt.want, tt.want)
			}
		})
	}
}

// Test error classification for auth errors
func TestClassifyError_Auth(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		err  error
		want bool
	}{{
		name: "HTTP 401 Unauthorized",
		err:  newStatusError(http.StatusUnauthorized, "401 Unauthorized"),
		want: true,
	}, {
		name: "HTTP 403 Forbidden",
		err:  newStatusError(http.StatusForbidden, "403 Forbidden"),
		want: true,
	}, {
		name: "Token expired",
		err:  errors.New("token expired"),
		want: true,
	}, {
		name: "HTTP 404 Not Found",
		err:  newStatusError(404, "404 Not Found"),
		want: false,
	}} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			classified := ClassifyTransportError(tt.err)
			isAuth := isAuthenticationError(classified, tt.err)

			if isAuth != tt.want {
				t.Errorf("isAuth = %v, want %v", isAuth, tt.want)
			}
		})
	}
}

func isAuthenticationError(classified, original error) bool {
	var hErr *HTTPStatusError
	if errors.As(classified, &hErr) {
		code := hErr.StatusCode
		if code == http.StatusUnauthorized || code == http.StatusForbidden {
			return true
		}
	}

	errMsg := strings.ToLower(original.Error())

	return strings.Contains(errMsg, "token expired")
}

// Test error wrapping preserves classification
func TestErrorWrapping(t *testing.T) {
	t.Parallel()

	baseErr := newStatusError(404, "404 Not Found")
	wrappedErr := fmt.Errorf("connection failed: %w", baseErr)

	classified := ClassifyTransportError(wrappedErr)
	if e := new(ProtocolError); errors.As(classified, &e) != true {
		t.Error("Wrapped protocol error should be classified as protocol error")
	}
}

// Test error synthesis for "not an MCP server"
func TestSynthesizeError_NotMCPServer(t *testing.T) {
	t.Parallel()

	streamableErr := newStatusError(404, "404 Not Found")
	sseErr := errors.New("unexpected EOF")

	synthesized := SynthesizeNotMCPServerError("https://example.com", streamableErr, sseErr)

	expectedMsg := "address is not an MCP server"

	if !errors.Is(synthesized, ErrNotMCPServer) {
		t.Errorf("Synthesized error should wrap ErrNotMCPServer")
	}

	errStr := synthesized.Error()
	if len(errStr) < len(expectedMsg) {
		t.Errorf("Synthesized error too short: %q", errStr)
	}
}

// Helper type for testing
type timeoutError struct{}

func (e *timeoutError) Error() string   { return "timeout" }
func (e *timeoutError) Timeout() bool   { return true }
func (e *timeoutError) Temporary() bool { return true }

func newStatusError(statusCode int, status string) *HTTPStatusError {
	return &HTTPStatusError{
		StatusCode:     statusCode,
		Status:         status,
		ResponseHeader: nil,
		URL:            "",
	}
}

func noSuchHostErr(domain string) *net.DNSError {
	return &net.DNSError{
		Err:         "no such host",
		Name:        domain,
		IsNotFound:  true,
		UnwrapErr:   nil,
		Server:      "",
		IsTimeout:   false,
		IsTemporary: false,
	}
}

func netOpError(op, network string, err error) *net.OpError {
	return &net.OpError{
		Op:   op,
		Net:  network,
		Addr: nil,
		Err:  err,
	}
}

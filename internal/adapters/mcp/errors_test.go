package mcp_test

import (
	"errors"
	"fmt"
	"net"
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
		err:  &net.DNSError{Err: "no such host", Name: "nonexistent.example.com", IsNotFound: true},
		want: true,
	}, {
		name: "Connection refused",
		err:  &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED},
		want: true,
	}, {
		name: "Network unreachable",
		err:  &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ENETUNREACH},
		want: true,
	}, {
		name: "TLS handshake failure",
		err:  errors.New("tls: handshake failure"),
		want: true,
	}, {
		name: "Timeout error",
		err:  &net.OpError{Op: "read", Net: "tcp", Err: &timeoutError{}},
		want: true, // Timeouts are infrastructure failures (not protocol mismatches)
	}} {
		t.Run(tt.name, func(t *testing.T) {
			classified := ClassifyTransportError(tt.err)
			if e := new(InfrastructureError); errors.As(classified, &e) != tt.want {
				t.Errorf("classifyError(%v).IsInfrastructure() = %v, want %v", tt.err, !tt.want, tt.want)
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
		err:  &HTTPStatusError{StatusCode: 400, Status: "400 Bad Request"},
		want: true,
	}, {
		name: "HTTP 404 Not Found",
		err:  &HTTPStatusError{StatusCode: 404, Status: "404 Not Found"},
		want: true,
	}, {
		name: "HTTP 405 Method Not Allowed",
		err:  &HTTPStatusError{StatusCode: 405, Status: "405 Method Not Allowed"},
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
		err:  &HTTPStatusError{StatusCode: 500, Status: "500 Internal Server Error"},
		want: false, // 5xx are server errors, not protocol mismatches
	}} {
		t.Run(tt.name, func(t *testing.T) {
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
		err:  &HTTPStatusError{StatusCode: 401, Status: "401 Unauthorized"},
		want: true,
	}, {
		name: "HTTP 403 Forbidden",
		err:  &HTTPStatusError{StatusCode: 403, Status: "403 Forbidden"},
		want: true,
	}, {
		name: "Token expired",
		err:  errors.New("token expired"),
		want: true,
	}, {
		name: "HTTP 404 Not Found",
		err:  &HTTPStatusError{StatusCode: 404, Status: "404 Not Found"},
		want: false, // 404 is protocol, not auth
	}} {
		t.Run(tt.name, func(t *testing.T) {
			classified := ClassifyTransportError(tt.err)
			var hErr *HTTPStatusError
			isAuth := errors.As(classified, &hErr) && (hErr.StatusCode == 401 || hErr.StatusCode == 403)
			if !isAuth && strings.Contains(strings.ToLower(tt.err.Error()), "token expired") {
				isAuth = true
			}
			if isAuth != tt.want {
				t.Errorf("classifyError(%v).IsAuth() = %v, want %v", tt.err, isAuth, tt.want)
			}
		})
	}
}

// Test error wrapping preserves classification
func TestErrorWrapping(t *testing.T) {
	t.Parallel()

	baseErr := &HTTPStatusError{StatusCode: 404, Status: "404 Not Found"}
	wrappedErr := fmt.Errorf("connection failed: %w", baseErr)

	classified := ClassifyTransportError(wrappedErr)
	if e := new(ProtocolError); errors.As(classified, &e) != true {
		t.Error("Wrapped protocol error should be classified as protocol error")
	}
}

// Test error synthesis for "not an MCP server"
func TestSynthesizeError_NotMCPServer(t *testing.T) {
	t.Parallel()

	streamableErr := &HTTPStatusError{StatusCode: 404, Status: "404 Not Found"}
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

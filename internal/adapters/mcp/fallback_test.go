package mcp_test

import (
	"errors"
	"testing"

	. "github.com/quenbyako/cynosure/internal/adapters/mcp"
)

// TestProtocolFallbackDecisionLogic tests the fallback decision logic
func TestProtocolFallbackDecisionLogic(t *testing.T) {
	for _, tt := range []struct {
		name           string
		firstErr       error
		shouldFallback bool
	}{{
		name:           "Protocol error triggers fallback",
		firstErr:       ErrProtocol(&HTTPStatusError{StatusCode: 404, Status: "Not Found"}),
		shouldFallback: true,
	}, {
		name:           "Infrastructure error does not trigger fallback",
		firstErr:       ErrInfrastructure(errors.New("connection refused")),
		shouldFallback: false,
	}, {
		name:           "Auth error does not trigger fallback",
		firstErr:       ErrAuth(&HTTPStatusError{StatusCode: 401, Status: "Unauthorized"}),
		shouldFallback: false,
	}, {
		name:           "Nil error does not trigger fallback",
		firstErr:       nil,
		shouldFallback: false,
	}} {
		t.Run(tt.name, func(t *testing.T) {
			e := new(ProtocolError)
			shouldFallback := tt.firstErr != nil && errors.As(tt.firstErr, &e)

			if shouldFallback != tt.shouldFallback {
				t.Errorf("Expected shouldFallback=%v, got %v", tt.shouldFallback, shouldFallback)
			}
		})
	}
}

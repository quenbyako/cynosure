package embedding

import (
	"errors"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

var (
	ErrHistoryTooLong     = messages.ErrInternalValidation("history is too long")
	ErrHardQuotaExhausted = errors.New("hard quota exhausted")
)

// PreflightFailedError is returned when a preflight check fails.
type PreflightFailedError struct {
	inner error
}

func (e *PreflightFailedError) Error() string {
	return "preflight failed: " + e.inner.Error()
}

func (e *PreflightFailedError) Unwrap() error { return e.inner }

// ErrPreflightFailed creates a new PreflightFailedError.
func ErrPreflightFailed(err error) *PreflightFailedError {
	return &PreflightFailedError{inner: err}
}

package chat

import (
	"fmt"
	"time"
)

type InternalValidationError string

func (e InternalValidationError) Error() string {
	return string(e)
}

func errInternalValidation(format string) error {
	return InternalValidationError(format)
}

type ToolIDNotPendingError struct {
	toolID string
}

func (e *ToolIDNotPendingError) Error() string {
	return fmt.Sprintf("unexpected tool result: ID %q is not pending in the current turn", e.toolID)
}

func (e *ToolIDNotPendingError) ToolID() string { return e.toolID }

func errToolIDNotPending(toolID string) *ToolIDNotPendingError {
	return &ToolIDNotPendingError{toolID: toolID}
}

// RateLimitExceededError occurs when the account has reached its assigned
// message quota.
type RateLimitExceededError struct {
	retryAt time.Time
}

func ErrRateLimitExceeded(next time.Time) *RateLimitExceededError {
	return &RateLimitExceededError{retryAt: next}
}

func (r *RateLimitExceededError) Error() string {
	return fmt.Sprintf(
		"rate limit exceeded, next request allowed at %v", r.retryAt,
	)
}

func (r *RateLimitExceededError) RetryAt() time.Time { return r.retryAt }

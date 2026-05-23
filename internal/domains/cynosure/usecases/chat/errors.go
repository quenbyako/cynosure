package chat

import (
	"errors"
	"fmt"
	"time"
)

var (
	// ErrNoAgentsFound is returned when no agents are available for a user.
	ErrNoAgentsFound = errors.New("no agents found")

	// ErrUnexpectedMessageType is returned when an unknown message type is encountered.
	ErrUnexpectedMessageType = errors.New("unexpected message type")

	// ErrEmptyReport is returned when a report is empty and cannot be written.
	ErrEmptyReport = errors.New("empty report, can't write to ratelimiter")

	errSettleFuncNotSet = errors.New("settle func wasn't set")
)

// InternalValidationError is returned when usecase configuration or parameters are invalid.
type InternalValidationError struct {
	Message string
}

func (e *InternalValidationError) Error() string {
	return "chat usecase validation error: " + e.Message
}

// errInternalValidation is a helper to create InternalValidationError.
func errInternalValidation(msg string, args ...any) error {
	return &InternalValidationError{Message: fmt.Sprintf(msg, args...)}
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

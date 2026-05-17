package users

import (
	"fmt"
	"time"
)

// InternalValidationError is returned when usecase configuration or parameters are invalid.
type InternalValidationError struct {
	Message string
}

func (e *InternalValidationError) Error() string {
	return "users usecase validation error: " + e.Message
}

// errInternalValidation is a helper to create InternalValidationError.
func errInternalValidation(msg string) error {
	return &InternalValidationError{Message: msg}
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

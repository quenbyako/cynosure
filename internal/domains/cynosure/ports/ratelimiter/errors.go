package ratelimiter

import (
	"fmt"
	"time"
)

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

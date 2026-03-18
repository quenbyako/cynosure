package ratelimiter

import (
	"errors"
)

// ErrRateLimitExceeded occurs when the account has reached its assigned
// message quota.
var ErrRateLimitExceeded = errors.New("rate limit exceeded")

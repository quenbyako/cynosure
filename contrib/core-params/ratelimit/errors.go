package ratelimit

import (
	"errors"
)

var (
	// ErrInvalidFormat is returned when the rate limit format is invalid.
	ErrInvalidFormat = errors.New("invalid rate limit format, expected burst/period (e.g. 20/1h)")

	// ErrInvalidBurst is returned when the burst is not positive.
	ErrInvalidBurst = errors.New("burst must be positive")
)

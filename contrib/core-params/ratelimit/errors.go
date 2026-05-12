package ratelimit

import (
	"errors"
)

var (
	// ErrInvalidFormat is returned when the rate limit format is invalid.
	ErrInvalidFormat = errors.New("invalid rate limit format, expected burst/period (e.g. 20/1h)")

	// ErrInvalidLimit is returned when the limit is not positive.
	ErrInvalidLimit = errors.New("limit must be positive")
)

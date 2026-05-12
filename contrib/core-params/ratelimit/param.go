// Package ratelimit provides rate limit parameter types.
package ratelimit

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

const (
	expectedParts = 2
)

// NewPolicy creates a new rate limit policy.
func NewPolicy(limit int, period time.Duration) Policy {
	return Policy{
		limit:  limit,
		period: period,
	}
}

// Policy defines a rate limit policy with a limit and burst.
// It implements encoding.TextUnmarshaler to allow parsing from strings like "20/1h".
//
//nolint:recvcheck // it's necessary to use value receiver to prevent modifying envs
type Policy struct {
	period time.Duration
	limit  int
}

// UnmarshalText implements encoding.TextUnmarshaler.
// Format: {burst}/{period} (e.g. "30/1m", "20/1h").
func (p *Policy) UnmarshalText(text []byte) error {
	str := string(text)
	if str == "" {
		return nil
	}

	parts := strings.Split(str, "/")
	if len(parts) != expectedParts {
		return ErrInvalidFormat
	}

	limit, err := strconv.Atoi(parts[0])
	if err != nil {
		return fmt.Errorf("invalid burst %q: %w", parts[0], err)
	}

	period, err := time.ParseDuration(parts[1])
	if err != nil {
		return fmt.Errorf("invalid period %q: %w", parts[1], err)
	}

	if limit <= 0 {
		return ErrInvalidLimit
	}

	p.limit = limit
	p.period = period

	return nil
}

func (p Policy) String() string {
	if p.period == 0 {
		return ""
	}

	return fmt.Sprintf("%d/%v", p.limit, p.period)
}

// Limit returns the burst capacity of the policy.
func (p Policy) Limit() int { return p.limit }

// Period returns the duration period of the policy.
func (p Policy) Period() time.Duration { return p.period }

// Rate returns the [rate.Limit] calculated from the policy.
func (p Policy) Rate() rate.Limit { return rate.Every(p.period / time.Duration(p.limit)) }

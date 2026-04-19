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

// Policy defines a rate limit policy with a limit and burst.
// It implements encoding.TextUnmarshaler to allow parsing from strings like "20/1h".
//
//nolint:recvcheck // it's necessary to use value receiver to prevent modifying envs
type Policy struct {
	period time.Duration
	burst  int
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

	burst, err := strconv.Atoi(parts[0])
	if err != nil {
		return fmt.Errorf("invalid burst %q: %w", parts[0], err)
	}

	period, err := time.ParseDuration(parts[1])
	if err != nil {
		return fmt.Errorf("invalid period %q: %w", parts[1], err)
	}

	if burst <= 0 {
		return ErrInvalidBurst
	}

	p.burst = burst
	p.period = period

	return nil
}

func (p Policy) String() string {
	if p.period == 0 {
		return ""
	}

	return fmt.Sprintf("%d/%v", p.burst, p.period)
}

// Burst returns the burst capacity of the policy.
func (p Policy) Burst() int { return p.burst }

// Period returns the duration period of the policy.
func (p Policy) Period() time.Duration { return p.period }

// Limit returns the rate.Limit calculated from the policy.
func (p Policy) Limit() rate.Limit { return rate.Every(p.period / time.Duration(p.burst)) }

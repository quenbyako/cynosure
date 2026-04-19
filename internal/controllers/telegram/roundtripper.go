package telegram

import (
	"fmt"
	"net/http"

	"golang.org/x/time/rate"
)

// rateLimitTransport wraps an http.RoundTripper and enforces a rate limit
// on the outgoing requests. This blocks until the token is available.
type rateLimitTransport struct {
	base    http.RoundTripper
	limiter *rate.Limiter
}

func (t *rateLimitTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := t.limiter.Wait(req.Context()); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}

	//nolint:wrapcheck // should not wrap standard library errors
	return t.base.RoundTrip(req)
}

// NewRateLimitTransport creates a new transport that complies with the given rate.Limiter.
func NewRateLimitTransport(base http.RoundTripper, limiter *rate.Limiter) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}

	if limiter == nil {
		return base
	}

	return &rateLimitTransport{
		base:    base,
		limiter: limiter,
	}
}

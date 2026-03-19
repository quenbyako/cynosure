// Package ratelimiter provides the interface for the rate limiter.
package ratelimiter

import (
	"context"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

// Port manages rate limiting of messages for given users.
type Port interface {
	// Consume consumes message quota for the given user.
	//
	// Throws:
	//
	//  - [ErrRateLimitExceeded] if user has reached its assigned quota.
	Consume(ctx context.Context, user ids.UserID, n int) error
}

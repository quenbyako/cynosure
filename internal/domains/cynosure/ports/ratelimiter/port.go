// Package ratelimiter provides the interface for the rate limiter.
package ratelimiter

import (
	"context"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

type ConsumedTokensFunc func(ctx context.Context, outputTokens int) error

// Port manages resource consumption, quotas, and access control for users.
//
// # Architectural Role
//
// This port serves as the "Accountant" and "Guard" of the system. It is
// responsible for tracking, calculating, and enforcing user-specific quotas
// across various operations. By encapsulating these rules, it decouples the
// domain logic from the financial, operational, and business costs of LLM
// usage.
//
// # Cost Normalization (Multipliers)
//
// Different models (e.g., "gemini-1.5-flash" vs "gemini-1.5-pro") have
// different resource costs. The Port's implementation (adapters) MUST handle
// the conversion of raw units (tokens, requests) into a normalized internal
// metric (e.g., "units"). This allows the domain to simply report "I used X
// tokens of model Y", while the adapter decides if the user has enough
// "balance" left based on current pricing.
type Port interface {
	// ConsumeChat consumes message quota for the given user and model.
	// It returns a callback to record token usage after the chat completes.
	//
	// Throws:
	//
	//  - [RateLimitExceededError] if user has reached its assigned quota.
	ConsumeChatRequests(ctx context.Context, user ids.UserID, model string, inputTokens int) (ConsumedTokensFunc, error)

	// ConsumeEmbeddingRequests consumes embedding requests quota for the given user.
	// It returns a callback to record token usage after the embedding completes.
	//
	// Throws:
	//
	//  - [RateLimitExceededError] if user has reached its assigned quota.
	ConsumeEmbeddingRequests(ctx context.Context, user ids.UserID, model string, inputTokens int) error
}

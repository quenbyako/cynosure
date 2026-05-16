package chatmodel

import (
	"context"
	"iter"
	"time"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

type (
	// StreamIter is an iterator of message chunks.
	StreamIter = iter.Seq2[messages.Message, error]

	// PreflightFunc is a function that checks if a request can be processed. It is
	// used to check for rate limits, token limits, etc. If the check fails, the
	// request will be aborted.
	PreflightFunc func(ctx context.Context, modelName string, inputTokens int) error

	UsageStats struct {
		InputTokens  uint32
		OutputTokens uint32
		Duration     time.Duration
	}

	Iter interface {
		Next() (messages.Message, bool)
		Close() (UsageStats, error)
	}
)

func noopPreflight(context.Context, string, int) error { return nil }

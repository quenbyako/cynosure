// Package chatmodel provides an interface for interacting with a chat model.
package chatmodel

import (
	"context"
	"iter"
	"time"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
)

// StreamIter is an iterator of message chunks.
type StreamIter = iter.Seq2[messages.Message, error]

// Port generates AI responses via LLM streaming API. Supports tool calling and
// custom agent parameters (temperature, system prompt, etc.).
//
// # Rate Limiting
//
// This port is subject to usage quotas. To prevent over usage of expensive API,
// each adapter may implement (or may not!) its own hard caps for usage. This
// behavior is a fallback, if domain layer of quota management will fail to do
// its job, e.g. if too many users will try to use the servise, global quota per
// each module may trigger by throwing [HardQuotaExhaustedError]. Note, that
// this error may happen only BEFORE request will be sent to the provider, it
// doesn't support ratelimit debt.
type Port interface {
	// Stream generates AI response as an iterator of message chunks. Supports
	// tool calling when toolbox is provided via StreamOption. Returns iterator
	// that yields message chunks and errors.
	//
	// Options:
	//
	//  - [WithStreamToolbox] — sets the toolbox for newly creating tools.
	//  - [WithStreamToolChoice] — sets the tool choice for newly creating tools.
	//
	// See next test suites to find how it works:
	//
	//  - [TestStreamBasicResponse] — generating simple text responses
	//  - [TestStreamWithTools] — tool calling flow and message formatting
	//
	// Throws:
	//
	//  - [ErrHistoryTooLong] if history is too long.
	//  - [HardQuotaExhaustedError] if hard quota is exhausted. See
	//    documentation for [Port] to get more about quotas and rate limiting.
	StreamWithStats(
		ctx context.Context,
		input []messages.Message,
		settings entities.AgentReadOnly,
		opts ...StreamOption,
	) (Iter, error)
}

func defaultStreamParams(required streamRequiredParams) streamParams {
	return streamParams{
		streamRequiredParams: required,
		tools:                tools.Toolbox{},
		toolChoice:           tools.ToolChoiceAllowed,
	}
}

func (s *streamParams) validate() error {
	return nil
}

type UsageStats struct {
	InputTokens  uint32
	OutputTokens uint32
	Duration     time.Duration
}

type Iter interface {
	Next() (messages.Message, bool)
	Close() (UsageStats, error)
}

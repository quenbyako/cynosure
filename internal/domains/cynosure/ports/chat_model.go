package ports

import (
	"context"
	"iter"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
)

// ChatModel generates AI responses via LLM streaming API. Supports tool
// calling and custom agent parameters (temperature, system prompt, etc.).
type ChatModel interface {
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
	Stream(ctx context.Context, input []messages.Message, settings entities.AgentReadOnly, opts ...StreamOption) (iter.Seq2[messages.Message, error], error)
}

func defaultStreamParams() *streamParams {
	return &streamParams{
		toolChoice: tools.ToolChoiceAllowed,
		tools:      tools.Toolbox{},
	}
}

type ChatModelFactory interface {
	ChatModel() ChatModel
}

func NewChatModel(factory ChatModelFactory) ChatModel { return factory.ChatModel() }

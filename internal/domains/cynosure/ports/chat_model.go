package ports

import (
	"context"
	"iter"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/messages"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/tools"
)

type StreamOption func(*streamParams)

type streamParams struct {
	toolChoice tools.ToolChoice
	tools      []tools.RawToolInfo
}

func StreamParams(opts ...StreamOption) *streamParams {
	p := &streamParams{
		toolChoice: tools.ToolChoiceAllowed,
		tools:      nil,
	}
	for _, opt := range opts {
		opt(p)
	}

	return p
}

func (s *streamParams) Tools() []tools.RawToolInfo   { return s.tools }
func (s *streamParams) ToolChoice() tools.ToolChoice { return s.toolChoice }

func WithStreamTools(tools []tools.RawToolInfo) StreamOption {
	return func(p *streamParams) { p.tools = tools }
}

func WithStreamToolChoice(choice tools.ToolChoice) StreamOption {
	return func(p *streamParams) { p.toolChoice = choice }
}

type ChatModel interface {
	Stream(ctx context.Context, input []messages.Message, settings entities.ModelSettingsReadOnly, opts ...StreamOption) (iter.Seq2[messages.Message, error], error)
}

type ChatModelFactory interface {
	ChatModel() ChatModel
}

func NewChatModel(factory ChatModelFactory) ChatModel { return factory.ChatModel() }

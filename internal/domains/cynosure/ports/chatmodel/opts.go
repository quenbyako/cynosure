package chatmodel

import (
	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
)

// WithStreamToolbox sets the toolbox for newly creating tools.
//
// Applies to:
//
//   - [ChatModel.Stream]
func WithStreamToolbox(toolbox tools.Toolbox) StreamOption {
	return streamFunc(func(p *streamParams) { p.tools = toolbox })
}

// WithStreamToolChoice sets the tool choice for newly creating tools.
//
// Applies to:
//
//   - [ChatModel.Stream]
func WithStreamToolChoice(choice tools.ToolChoice) StreamOption {
	return streamFunc(func(p *streamParams) { p.toolChoice = choice })
}

type (
	StreamOption interface{ applyStream(p *streamParams) }

	streamFunc func(*streamParams)
)

var _ StreamOption = streamFunc(nil)

func (f streamFunc) applyStream(p *streamParams) { f(p) }

// ========================================================================== //
//                              [Port.Stream]                                 //
// ========================================================================== //

type streamRequiredParams struct {
	settings entities.AgentReadOnly
	input    []messages.Message
}

type streamParams struct {
	tools tools.Toolbox
	streamRequiredParams
	toolChoice tools.ToolChoice
}

func StreamParams(
	input []messages.Message, settings entities.AgentReadOnly, opts ...StreamOption,
) (streamParams, error) {
	params := defaultStreamParams(streamRequiredParams{
		input:    input,
		settings: settings,
	})
	for _, opt := range opts {
		opt.applyStream(&params)
	}

	if err := params.validate(); err != nil {
		return streamParams{}, err
	}

	return params, nil
}

func (s *streamParams) Input() []messages.Message        { return s.input }
func (s *streamParams) Settings() entities.AgentReadOnly { return s.settings }
func (s *streamParams) Toolbox() tools.Toolbox           { return s.tools }
func (s *streamParams) ToolChoice() tools.ToolChoice     { return s.toolChoice }

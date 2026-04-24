package chatmodel

import (
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

type streamParams struct {
	tools      tools.Toolbox
	toolChoice tools.ToolChoice
}

// ========================================================================== //
//                            [ChatModel.Stream]                              //
// ========================================================================== //

func StreamParams(opts ...StreamOption) streamParams {
	p := defaultStreamParams()
	for _, opt := range opts {
		opt.applyStream(&p)
	}

	return p
}

func (s *streamParams) Toolbox() tools.Toolbox       { return s.tools }
func (s *streamParams) ToolChoice() tools.ToolChoice { return s.toolChoice }

package ports

import (
	"go.opentelemetry.io/otel/trace"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
)

// Unlike common option that expects TracerProvider, this option expects
// initialized metrics provider, that will be converted into Observable.
//
// Applies to:
//
//   - [WrapThreadStorage]
func WithTrace(trace trace.Tracer) traceWrapper {
	return traceWrapper{trace: trace}
}

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
	StreamOption            interface{ applyStream(*streamParams) }
	WrapThreadStorageOption interface{ applyWrapThreadStorage(*threadStorageWrapped) }

	streamFunc func(*streamParams)

	traceWrapper struct{ trace trace.Tracer }
)

var (
	_ StreamOption = (streamFunc)(nil)

	_ WrapThreadStorageOption = traceWrapper{}
)

func (f streamFunc) applyStream(p *streamParams) { f(p) }

func (t traceWrapper) applyWrapThreadStorage(p *threadStorageWrapped) { p.trace = t.trace }

//============================================================================//
//                            [ChatModel.Stream]                             //
//============================================================================//

type streamParams struct {
	toolChoice tools.ToolChoice
	tools      tools.Toolbox
}

func StreamParams(opts ...StreamOption) *streamParams {
	p := defaultStreamParams()
	for _, opt := range opts {
		opt.applyStream(p)
	}

	return p
}

func (s *streamParams) Toolbox() tools.Toolbox       { return s.tools }
func (s *streamParams) ToolChoice() tools.ToolChoice { return s.toolChoice }

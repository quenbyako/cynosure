package ports

import (
	"net/url"

	"go.opentelemetry.io/otel/trace"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
)

// WithSuggestedLink suggests a link to use for oauth client registration.
//
// Applies to:
//
//   - [OAuthHandler.RegisterClient]
func WithSuggestedProtectedResource(link *url.URL) RegisterClientOption {
	return registerClientFunc(func(p *registerClientParams) { p.suggestedProtectedResource = link })
}

// WithToolIDBuilder sets the tool ID builder for newly creating tools.
//
// Applies to:
//
//   - [ToolClient.DiscoverTools]
func WithToolIDBuilder(builder func(account ids.AccountID, name string) (ids.ToolID, error)) DiscoverToolsOption {
	return discoverToolsFunc(func(p *discoverToolsParams) { p.toolIDBuilder = builder })
}

// Unlike common option that expects TracerProvider, this option expects
// initialized tracer, cause traces must show REAL package name, instead of
// wrapper.
//
// Applies to:
//
//   - [WrapThreadStorage]
//   - [WrapIdentityManager]
//   - [WrapToolClient]
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
	DiscoverToolsOption       interface{ applyDiscoverTools(*discoverToolsParams) }
	RegisterClientOption      interface{ applyRegisterClient(*registerClientParams) }
	StreamOption              interface{ applyStream(*streamParams) }
	WrapThreadStorageOption   interface{ applyWrapThreadStorage(*threadStorageWrapped) }
	WrapIdentityManagerOption interface{ applyWrapIdentityManager(*identityManagerWrapped) }
	WrapToolClientOption      interface{ applyWrapToolClient(*toolClientWrapped) }

	streamFunc         func(*streamParams)
	registerClientFunc func(*registerClientParams)
	discoverToolsFunc  func(*discoverToolsParams)

	traceWrapper struct{ trace trace.Tracer }
)

var (
	_ DiscoverToolsOption  = (discoverToolsFunc)(nil)
	_ RegisterClientOption = (registerClientFunc)(nil)
	_ StreamOption         = (streamFunc)(nil)

	_ WrapThreadStorageOption   = traceWrapper{}
	_ WrapIdentityManagerOption = traceWrapper{}
	_ WrapToolClientOption      = traceWrapper{}
)

func (f discoverToolsFunc) applyDiscoverTools(p *discoverToolsParams)    { f(p) }
func (f registerClientFunc) applyRegisterClient(p *registerClientParams) { f(p) }
func (f streamFunc) applyStream(p *streamParams)                         { f(p) }

func (t traceWrapper) applyWrapThreadStorage(p *threadStorageWrapped)     { p.trace = t.trace }
func (t traceWrapper) applyWrapIdentityManager(p *identityManagerWrapped) { p.trace = t.trace }
func (t traceWrapper) applyWrapToolClient(p *toolClientWrapped)           { p.trace = t.trace }

//============================================================================//
//                        [OAuthHandler.RegisterClient]                       //
//============================================================================//

type registerClientParams struct {
	suggestedProtectedResource *url.URL
}

// RegisterClientParams creates a new set of parameters for [OAuthHandler.RegisterClient].
func RegisterClientParams(opts ...RegisterClientOption) *registerClientParams {
	p := defaultRegisterClientParams()
	for _, opt := range opts {
		opt.applyRegisterClient(p)
	}

	return p
}

func (s *registerClientParams) SuggestedProtectedResource() *url.URL {
	return s.suggestedProtectedResource
}

//============================================================================//
//                          [ToolClient.DiscoverTools]                        //
//============================================================================//

type discoverToolsParams struct {
	toolIDBuilder func(account ids.AccountID, name string) (ids.ToolID, error)
}

func DiscoverToolsParams(opts ...DiscoverToolsOption) *discoverToolsParams {
	p := defaultDiscoverToolsParams()
	for _, opt := range opts {
		opt.applyDiscoverTools(p)
	}

	return p
}

func (s *discoverToolsParams) ToolIDBuilder() func(account ids.AccountID, name string) (ids.ToolID, error) {
	return s.toolIDBuilder
}

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

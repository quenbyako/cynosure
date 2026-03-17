package toolclient

import (
	"golang.org/x/oauth2"
)

// WithToolIDBuilder sets the tool ID builder for newly creating tools.
//
// Applies to:
//
//   - [ToolClient.DiscoverTools]
func WithToolIDBuilder(builder ToolIDBuilder) DiscoverToolsOption {
	return discoverToolsFunc(func(p *discoverToolsParams) { p.toolIDBuilder = builder })
}

// WithAuthToken sets the auth token for newly creating tools.
//
// Applies to:
//
//   - [ToolClient.DiscoverTools]
func WithAuthToken(token *oauth2.Token) DiscoverToolsOption {
	return discoverToolsFunc(func(p *discoverToolsParams) { p.token = token })
}

type (
	DiscoverToolsOption interface{ applyDiscoverTools(p *discoverToolsParams) }

	discoverToolsFunc func(*discoverToolsParams)
)

var _ DiscoverToolsOption = discoverToolsFunc(nil)

func (f discoverToolsFunc) applyDiscoverTools(p *discoverToolsParams) { f(p) }

// ========================================================================== //
//                          [ToolClient.DiscoverTools]                        //
// ========================================================================== //

type discoverToolsParams struct {
	toolIDBuilder ToolIDBuilder
	token         *oauth2.Token
}

func DiscoverToolsParams(opts ...DiscoverToolsOption) discoverToolsParams {
	p := defaultDiscoverToolsParams()
	for _, opt := range opts {
		opt.applyDiscoverTools(&p)
	}

	return p
}

func resolvedDiscoverToolsParams(value discoverToolsParams) DiscoverToolsOption {
	return discoverToolsFunc(func(p *discoverToolsParams) { *p = value })
}

func (s *discoverToolsParams) ToolIDBuilder() ToolIDBuilder {
	return s.toolIDBuilder
}

func (s *discoverToolsParams) Token() *oauth2.Token {
	return s.token
}

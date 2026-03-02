package toolclient

import (
	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

// WithToolIDBuilder sets the tool ID builder for newly creating tools.
//
// Applies to:
//
//   - [ToolClient.DiscoverTools]
func WithToolIDBuilder(builder func(account ids.AccountID, name string) (ids.ToolID, error)) DiscoverToolsOption {
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
	DiscoverToolsOption interface{ applyDiscoverTools(*discoverToolsParams) }

	discoverToolsFunc func(*discoverToolsParams)
)

var (
	_ DiscoverToolsOption = (discoverToolsFunc)(nil)
)

func (f discoverToolsFunc) applyDiscoverTools(p *discoverToolsParams) { f(p) }

//============================================================================//
//                          [ToolClient.DiscoverTools]                        //
//============================================================================//

type discoverToolsParams struct {
	toolIDBuilder func(account ids.AccountID, name string) (ids.ToolID, error)
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

func (s *discoverToolsParams) ToolIDBuilder() func(account ids.AccountID, name string) (ids.ToolID, error) {
	return s.toolIDBuilder
}

func (s *discoverToolsParams) Token() *oauth2.Token {
	return s.token
}

package ports

import (
	"net/url"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

// WithSuggestedLink suggests a link to use for oauth client registration.
func WithSuggestedLink(link *url.URL) RegisterClientOption {
	return registerClientFunc(func(p *registerClientParams) { p.suggestedLink = link })
}

// WithToolIDBuilder sets the tool ID builder for newly creating tools.
func WithToolIDBuilder(builder func(account ids.AccountID, name string) (ids.ToolID, error)) DiscoverToolsOption {
	return discoverToolsFunc(func(p *discoverToolsParams) { p.toolIDBuilder = builder })
}

type (
	RegisterClientOption interface{ applyRegisterClient(*registerClientParams) }
	DiscoverToolsOption  interface{ applyDiscoverTools(*discoverToolsParams) }

	registerClientFunc func(*registerClientParams)
	discoverToolsFunc  func(*discoverToolsParams)
)

var (
	_ RegisterClientOption = (registerClientFunc)(nil)
	_ DiscoverToolsOption  = (discoverToolsFunc)(nil)
)

func (f registerClientFunc) applyRegisterClient(p *registerClientParams) { f(p) }
func (f discoverToolsFunc) applyDiscoverTools(p *discoverToolsParams)    { f(p) }

//============================================================================//
//                        [OAuthHandler.RegisterClient]                       //
//============================================================================//

type registerClientParams struct {
	suggestedLink *url.URL
}

// RegisterClientParams creates a new set of parameters for [OAuthHandler.RegisterClient].
func RegisterClientParams(opts ...RegisterClientOption) *registerClientParams {
	p := &registerClientParams{
		suggestedLink: nil,
	}
	for _, opt := range opts {
		opt.applyRegisterClient(p)
	}

	return p
}

func (s *registerClientParams) SuggestedLink() *url.URL { return s.suggestedLink }

//============================================================================//
//                          [ToolClient.DiscoverTools]                        //
//============================================================================//

type discoverToolsParams struct {
	toolIDBuilder func(account ids.AccountID, name string) (ids.ToolID, error)
}

func DiscoverToolsParams(opts ...DiscoverToolsOption) *discoverToolsParams {
	p := &discoverToolsParams{
		toolIDBuilder: func(account ids.AccountID, name string) (ids.ToolID, error) {
			return ids.RandomToolID(account, ids.WithSlug(name))
		},
	}
	for _, opt := range opts {
		opt.applyDiscoverTools(p)
	}

	return p
}

func (s *discoverToolsParams) ToolIDBuilder() func(account ids.AccountID, name string) (ids.ToolID, error) {
	return s.toolIDBuilder
}

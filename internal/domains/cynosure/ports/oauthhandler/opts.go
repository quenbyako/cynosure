package oauthhandler

import (
	"net/url"
)

// WithSuggestedProtectedResource suggests a link to use for oauth client registration.
//
// Applies to:
//
//   - [Port.RegisterClient]
func WithSuggestedProtectedResource(link *url.URL) RegisterClientOption {
	return registerClientFunc(func(p *registerClientParams) { p.suggestedProtectedResource = link })
}

type (
	RegisterClientOption interface{ applyRegisterClient(*registerClientParams) }

	registerClientFunc func(*registerClientParams)
)

var _ RegisterClientOption = (registerClientFunc)(nil)

func (f registerClientFunc) applyRegisterClient(p *registerClientParams) { f(p) }

// ========================================================================== //
//                        [OAuthHandler.RegisterClient]                       //
// ========================================================================== //

type registerClientParams struct {
	suggestedProtectedResource *url.URL
}

// RegisterClientParams creates a new set of parameters for [Port.RegisterClient].
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

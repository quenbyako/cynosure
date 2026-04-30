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

// WithInternalConnection explicitly defines, that request may be sent to
// internal network. By default, all ports must ensure that they are safe to
// SSRF attacks. This option tells port that accessing local infrastructure is
// allowed.
//
// Applies to:
//
//   - [Port.RefreshToken]
//   - [Port.Exchange]
func WithInternalConnection() internalConnectionOption {
	return internalConnectionOption{}
}

type internalConnectionOption struct{}

func (i internalConnectionOption) applyRegisterClient(p *registerClientParams) { p.internal = true }
func (i internalConnectionOption) applyRefreshToken(p *refreshTokenParams)     { p.internal = true }

// ========================================================================== //
//                                [types]                                     //
// ========================================================================== //

type (
	RegisterClientOption interface{ applyRegisterClient(p *registerClientParams) }
	RefreshTokenOption   interface{ applyRefreshToken(p *refreshTokenParams) }

	registerClientFunc func(*registerClientParams)
	refreshTokenFunc   func(*refreshTokenParams)
)

var (
	_ RegisterClientOption = registerClientFunc(nil)
	_ RefreshTokenOption   = refreshTokenFunc(nil)
)

func (f registerClientFunc) applyRegisterClient(p *registerClientParams) { f(p) }
func (f refreshTokenFunc) applyRefreshToken(p *refreshTokenParams)       { f(p) }

// ========================================================================== //
//                           [Port.RegisterClient]                            //
// ========================================================================== //

type registerClientParams struct {
	suggestedProtectedResource *url.URL
	internal                   bool
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

func (s *registerClientParams) Internal() bool { return s.internal }

// ========================================================================== //
//                            [Port.RefreshToken]                             //
// ========================================================================== //

type refreshTokenParams struct {
	internal bool
}

// RefreshTokenParams creates a new set of parameters for [Port.RefreshToken].
func RefreshTokenParams(opts ...RefreshTokenOption) *refreshTokenParams {
	p := defaultRefreshTokenParams()
	for _, opt := range opts {
		opt.applyRefreshToken(p)
	}

	return p
}

func (p *refreshTokenParams) Internal() bool { return p.internal }

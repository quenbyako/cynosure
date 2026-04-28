package mcp

import (
	"net/http"

	"github.com/quenbyako/core"
)

type handlerParams struct {
	traceProvider core.Metrics
	// external and internal transports are required, but since they have same
	// type, we can't provide them through arguments.
	externalTransport http.RoundTripper
	internalTransport http.RoundTripper
	maxConnSize       uint
	// If true, the external client will not have any protection against SSRF.
	// This is useful for development and testing, but should not be used in
	// production.
	unsafeExternalClient bool
}

type HandlerOption func(*handlerParams)

func WithObservability(tp core.Metrics) HandlerOption {
	return func(p *handlerParams) { p.traceProvider = tp }
}

func WithMaxConnSize(size uint) HandlerOption {
	return func(p *handlerParams) { p.maxConnSize = size }
}

func WithUnsafeExternalHTTPClient(client http.RoundTripper) HandlerOption {
	return func(p *handlerParams) { p.externalTransport, p.unsafeExternalClient = client, true }
}

func WithExternalHTTPClient(client http.RoundTripper) HandlerOption {
	return func(p *handlerParams) { p.externalTransport, p.unsafeExternalClient = client, false }
}

func WithInternalHTTPClient(client http.RoundTripper) HandlerOption {
	return func(p *handlerParams) { p.internalTransport = client }
}

func buildHandlerParams(opts ...HandlerOption) handlerParams {
	params := handlerParams{
		traceProvider: core.NoopMetrics(),
		maxConnSize:   defaultMaxConnSize,
		// both transports are nil to help validator detect not setted
		// transport.
		externalTransport:    nil,
		internalTransport:    nil,
		unsafeExternalClient: false,
	}

	for _, opt := range opts {
		opt(&params)
	}

	return params
}

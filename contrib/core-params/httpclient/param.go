// Package httpclient provides an industrial-grade HTTP client parameter for the core library.
package httpclient

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"

	"github.com/quenbyako/core"
	"github.com/quenbyako/cynosure/contrib/core-params/httpclient/ssrf"
	"github.com/quenbyako/cynosure/contrib/core-params/ratelimit"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/time/rate"
)

const (
	defaultDialTimeout           = 30 * time.Second
	defaultKeepAlive             = 30 * time.Second
	defaultMaxIdleConns          = 100
	defaultIdleConnTimeout       = 90 * time.Second
	defaultTLSHandshakeTimeout   = 10 * time.Second
	defaultExpectContinueTimeout = 1 * time.Second
)

// Client provides a high-level HTTP transport interface with built-in
// observability, timeouts, and rate limiting, configured via environment variables.
type Client interface {
	http.RoundTripper
}

//nolint:gochecknoinits // core library uses init for parser registration
func init() { core.RegisterEnvParser(parseHTTPClient) }

type httpClientWrapper struct {
	addr             *url.URL
	chain            http.RoundTripper
	closeConnections func()
	timeout          time.Duration
	rateLimit        ratelimit.Policy
	ssrf             bool
}

var (
	_ core.EnvParam     = (*httpClientWrapper)(nil)
	_ http.RoundTripper = (*httpClientWrapper)(nil)
)

//nolint:ireturn // Client will be used in environment list.
func parseHTTPClient(_ context.Context, rawContent string) (Client, error) {
	parsedURL, err := url.Parse(rawContent)
	if err != nil {
		return nil, fmt.Errorf("parsing http client url: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, ErrUnsupportedScheme(parsedURL.Scheme)
	}

	wrapper := newDefaultWrapper(parsedURL)

	if parsedURL.Fragment != "" {
		if err := parseFragment(wrapper, parsedURL.Fragment); err != nil {
			return nil, err
		}
	}

	return wrapper, nil
}

func newDefaultWrapper(addr *url.URL) *httpClientWrapper {
	//nolint:exhaustruct // default dialer
	defaultDialer := &net.Dialer{
		Timeout:   defaultDialTimeout,
		KeepAlive: defaultKeepAlive,
	}

	//nolint:exhaustruct // too many optional parameters
	defaultTransport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           defaultDialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          defaultMaxIdleConns,
		IdleConnTimeout:       defaultIdleConnTimeout,
		TLSHandshakeTimeout:   defaultTLSHandshakeTimeout,
		ExpectContinueTimeout: defaultExpectContinueTimeout,
	}

	return &httpClientWrapper{
		addr:             addr,
		timeout:          0,
		rateLimit:        ratelimit.Policy{},
		chain:            defaultTransport,
		closeConnections: func() {},
		ssrf:             false,
	}
}

func parseFragment(wrapper *httpClientWrapper, fragment string) error {
	params, err := url.ParseQuery(fragment)
	if err != nil {
		return fmt.Errorf("parsing http client config from fragment: %w", err)
	}

	if t := params.Get("timeout"); t != "" {
		d, err := time.ParseDuration(t)
		if err != nil {
			return fmt.Errorf("invalid timeout %q: %w", t, err)
		}

		wrapper.timeout = d
	}

	if r := params.Get("rate"); r != "" {
		if err := wrapper.rateLimit.UnmarshalText([]byte(r)); err != nil {
			return fmt.Errorf("invalid rate limit %q: %w", r, err)
		}
	}

	if s := params.Get("ssrf"); s == "true" {
		wrapper.ssrf = true
	}

	return nil
}

func (h *httpClientWrapper) Configure(_ context.Context, data *core.ConfigureData) error {
	var control func(network, address string, c syscall.RawConn) error

	if h.ssrf {
		control = ssrf.New(ssrf.WithAnyPort()).Safe
	}

	//nolint:exhaustruct // standard dialer
	dialer := &net.Dialer{
		Timeout:   defaultDialTimeout,
		KeepAlive: defaultKeepAlive,
		Control:   control,
	}

	//nolint:exhaustruct // too many optional parameters
	base := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		//nolint:exhaustruct // standard dialer
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          defaultMaxIdleConns,
		IdleConnTimeout:       defaultIdleConnTimeout,
		TLSHandshakeTimeout:   defaultTLSHandshakeTimeout,
		ExpectContinueTimeout: defaultExpectContinueTimeout,
	}

	h.chain = h.buildTransport(base, data.Metric, data.Trace)
	h.closeConnections = base.CloseIdleConnections

	return nil
}

func (h *httpClientWrapper) buildTransport(
	base *http.Transport,
	meter metric.MeterProvider,
	tracer trace.TracerProvider,
) http.RoundTripper {
	var transport http.RoundTripper = base

	if h.rateLimit.Burst() > 0 && h.rateLimit.Limit() > 0 {
		transport = NewRateLimitTransport(
			transport,
			rate.NewLimiter(h.rateLimit.Limit(), h.rateLimit.Burst()),
		)
	}

	transport = otelhttp.NewTransport(
		transport,
		otelhttp.WithMeterProvider(meter),
		otelhttp.WithTracerProvider(tracer),
	)

	return transport
}

func (h *httpClientWrapper) Acquire(_ context.Context, _ *core.AcquireData) error {
	return nil
}

func (h *httpClientWrapper) Shutdown(_ context.Context, _ *core.ShutdownData) error {
	if h.closeConnections != nil {
		h.closeConnections()
	}

	return nil
}

func (h *httpClientWrapper) RoundTrip(req *http.Request) (*http.Response, error) {
	if h.chain == nil {
		return nil, errUnconfigured
	}

	ctx, cancel := h.prepareContext(req.Context())
	if cancel != nil {
		defer cancel()
	}

	out := req.WithContext(ctx)
	h.prepareURL(out)

	//nolint:wrapcheck // should not wrap transport errors.
	return h.chain.RoundTrip(out)
}

func (h *httpClientWrapper) prepareContext(
	ctx context.Context,
) (context.Context, context.CancelFunc) {
	if h.timeout > 0 {
		return context.WithTimeout(ctx, h.timeout)
	}

	return ctx, nil
}

func (h *httpClientWrapper) prepareURL(out *http.Request) {
	if out.URL.Host != "" {
		return
	}

	out.URL.Scheme = h.addr.Scheme
	out.URL.Host = h.addr.Host

	basePath := strings.TrimSuffix(h.addr.Path, "/")
	reqPath := strings.TrimPrefix(out.URL.Path, "/")

	switch {
	case reqPath != "":
		out.URL.Path = basePath + "/" + reqPath
	case basePath != "":
		out.URL.Path = basePath
	default:
		out.URL.Path = "/"
	}
}

// RateLimitTransport provides a RoundTripper that enforces rate limiting.

type rateLimitTransport struct {
	base    http.RoundTripper
	limiter *rate.Limiter
}

func (t *rateLimitTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := t.limiter.Wait(req.Context()); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}

	//nolint:wrapcheck // implementing interface
	return t.base.RoundTrip(req)
}

func NewRateLimitTransport(base http.RoundTripper, limiter *rate.Limiter) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}

	if limiter == nil {
		return base
	}

	return &rateLimitTransport{
		base:    base,
		limiter: limiter,
	}
}

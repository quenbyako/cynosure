// Package httpclient provides an industrial-grade HTTP client parameter for the core library.
package httpclient

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/quenbyako/core"
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
	addr      *url.URL
	base      *http.Transport
	chain     http.RoundTripper
	timeout   time.Duration
	rateLimit ratelimit.Policy
}

var (
	_ core.EnvParam     = (*httpClientWrapper)(nil)
	_ http.RoundTripper = (*httpClientWrapper)(nil)
)

func parseHTTPClient(_ context.Context, rawContent string) (Client, error) {
	parsedURL, err := url.Parse(rawContent)
	if err != nil {
		return nil, fmt.Errorf("parsing http client url: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		msg := "unsupported http client scheme %q"

		return nil, fmt.Errorf(msg, parsedURL.Scheme) //nolint:err113 // industrial param
	}

	var defaultBase *http.Transport
	if t, ok := http.DefaultTransport.(*http.Transport); ok {
		defaultBase = t.Clone()
	}

	wrapper := &httpClientWrapper{
		addr:      parsedURL,
		timeout:   0,
		rateLimit: ratelimit.Policy{},
		base:      defaultBase,
		chain:     defaultBase,
	}

	if parsedURL.Fragment != "" {
		if err := parseFragment(wrapper, parsedURL.Fragment); err != nil {
			return nil, err
		}
	}

	return wrapper, nil
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

	return nil
}

func (h *httpClientWrapper) Configure(_ context.Context, data *core.ConfigureData) error {
	//nolint:exhaustruct // too many optional parameters
	h.base = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		//nolint:exhaustruct // standard dialer
		DialContext: (&net.Dialer{
			Timeout:   defaultDialTimeout,
			KeepAlive: defaultKeepAlive,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          defaultMaxIdleConns,
		IdleConnTimeout:       defaultIdleConnTimeout,
		TLSHandshakeTimeout:   defaultTLSHandshakeTimeout,
		ExpectContinueTimeout: defaultExpectContinueTimeout,
	}

	h.chain = h.buildTransport(data.Metric, data.Trace)

	return nil
}

func (h *httpClientWrapper) buildTransport(
	meter metric.MeterProvider,
	tracer trace.TracerProvider,
) http.RoundTripper {
	var transport http.RoundTripper = h.base

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
	if h.base == nil {
		return nil
	}

	h.base.CloseIdleConnections()

	return nil
}

func (h *httpClientWrapper) RoundTrip(req *http.Request) (*http.Response, error) {
	if h.chain == nil {
		return nil, fmt.Errorf("http client not configured: Configure() must be called before RoundTrip()") //nolint:err113 // sentinel-free guard
	}

	ctx := req.Context()

	var cancel context.CancelFunc

	if h.timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, h.timeout)
		defer cancel()
	}

	out := req.WithContext(ctx)

	if out.URL.Host == "" {
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

	//nolint:wrapcheck // implementing interface
	return h.chain.RoundTrip(out)
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

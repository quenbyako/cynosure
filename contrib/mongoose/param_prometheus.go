package goose

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/quenbyako/cynosure/contrib/onelog"
)

type PromhttpExporter interface {
	Serve(ctx context.Context) error
	_PromhttpExporter()
}

type promhttpWrapper struct {
	log  onelog.Logger
	addr net.Addr

	conn net.Listener

	srv *http.Server
}

var _ configureType = (*promhttpWrapper)(nil)
var _ PromhttpExporter = (*promhttpWrapper)(nil)

func (g *promhttpWrapper) _PromhttpExporter() {}

func parsePromhttpExporter(c *[]configureType) env.ParserFunc {
	return func(v string) (any, error) {
		u, err := url.Parse(v)
		if err != nil {
			return nil, err
		}

		if u.Scheme != "http" {
			return nil, fmt.Errorf("unsupported HTTP scheme %q", u.Scheme)
		}

		host := u.Hostname()
		ip := net.ParseIP(host)
		if ip == nil {
			return nil, fmt.Errorf("invalid HTTP host %q", host)
		}
		port := u.Port()
		if port == "" {
			return nil, fmt.Errorf("invalid HTTP port %q", port)
		}
		portNum, err := strconv.Atoi(port)
		if err != nil {
			return nil, fmt.Errorf("invalid HTTP port %q", port)
		}
		if portNum < 1 || portNum > 65535 {
			return nil, fmt.Errorf("out of range HTTP port %q", port)
		}
		addr := &net.TCPAddr{IP: ip, Port: portNum}

		w := &promhttpWrapper{
			addr: addr,
			srv:  newHTTPServer(),
		}
		*c = append(*c, w)
		return w, nil
	}
}

func (g *promhttpWrapper) configure(ctx context.Context, data *configureData) error {
	g.log = onelog.Wrap(data.logger)
	g.srv.Handler = healthChecks(data.gatherer, nil)

	return nil
}

func (g *promhttpWrapper) acquire(ctx context.Context, data *acquireData) error {
	var err error
	g.conn, err = net.Listen(g.addr.Network(), g.addr.String()) // TODO: handle error correctly
	if err != nil {
		return fmt.Errorf("listening on %q %q: %w", g.addr.Network(), g.addr.String(), err)
	}

	return nil
}

func (h *promhttpWrapper) Serve(ctx context.Context) error {
	if h.conn == nil {
		panic("connection is not acquired")
	}

	if h.srv.Handler == nil {
		h.srv.Handler = http.HandlerFunc(http.NotFound)
	}

	stopLocker := make(chan struct{})
	var shutdownErr error
	go func(err *error) {
		defer close(stopLocker)
		<-ctx.Done()

		timeoutCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		*err = h.srv.Shutdown(timeoutCtx)
	}(&shutdownErr)

	h.log.Info().Str("addr", h.addr.String()).Msg("starting metrics server")

	err := h.srv.Serve(h.conn)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serving metrics: %w", err)
	}

	<-stopLocker

	h.log.Info().Str("addr", h.addr.String()).Msg("stopped metrics server")

	return shutdownErr
}

func (h *promhttpWrapper) shutdown(ctx context.Context, data *shutdownData) error {
	err := h.conn.Close()
	if err != nil {
		// poll.errNetClosing{}
		if err.Error() != "use of closed network connection" {
			return nil
		}

		return fmt.Errorf("closing connection: %w", err)
	}

	return nil
}

func healthChecks(promRegister prometheus.Gatherer, ready func(context.Context) bool) http.Handler {
	m := http.NewServeMux()
	m.Handle("/livez", livez())
	m.Handle("/readyz", readyz(ready))
	m.Handle("/metrics", promhttp.HandlerFor(promRegister, promhttp.HandlerOpts{}))

	return m
}

func livez() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func readyz(ready func(context.Context) bool) http.Handler {
	if ready == nil {
		ready = func(context.Context) bool { return true }
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ready(r.Context()) {
			w.WriteHeader(http.StatusOK)
			return
		}

		w.WriteHeader(http.StatusServiceUnavailable)
	})
}

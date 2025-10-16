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
	"github.com/quenbyako/cynosure/contrib/onelog"
)

const (
	DefaultReadHeaderTimeout = 5 * time.Second
	DefaultReadTimeout       = 5 * time.Second
	DefaultWriteTimeout      = 10 * time.Second
	DefaultIdleTimeout       = 120 * time.Second

	DefaultServerStopTimeout = 5 * time.Second
)

type HTTPServer interface {
	Register(http.Handler)

	Serve(ctx context.Context) error
}

type httpServerWrapper struct {
	log  onelog.Logger
	addr net.Addr

	conn net.Listener

	srv *http.Server
}

var _ configureType = (*httpServerWrapper)(nil)
var _ HTTPServer = (*httpServerWrapper)(nil)

func parseHTTPServer(c *[]configureType) env.ParserFunc {
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

		w := &httpServerWrapper{
			addr: addr,
			srv:  newHTTPServer(),
		}
		*c = append(*c, w)
		return w, nil
	}
}

func (g *httpServerWrapper) configure(ctx context.Context, data *configureData) error {
	g.log = onelog.Wrap(data.logger)

	return nil
}

func (g *httpServerWrapper) acquire(ctx context.Context, data *acquireData) error {
	var err error
	g.conn, err = net.Listen(g.addr.Network(), g.addr.String()) // TODO: handle error correctly
	if err != nil {
		return fmt.Errorf("listening on %q %q: %w", g.addr.Network(), g.addr.String(), err)
	}

	return nil
}

func (h *httpServerWrapper) Register(handler http.Handler) {
	// NOTE(rcooper): makes no sense to make this thread-safe, because
	// initialization usually performs in one goroutine.
	if h.srv.Handler != nil {
		panic("already registered")
	}

	h.srv.Handler = handler
}

func (h *httpServerWrapper) Serve(ctx context.Context) error {
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

	h.log.Info().Str("addr", h.addr.String()).Msg("starting HTTP server")

	err := h.srv.Serve(h.conn)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serving server: %w", err)
	}

	<-stopLocker

	h.log.Info().Str("addr", h.addr.String()).Msg("stopped HTTP server")

	return shutdownErr
}

func (h *httpServerWrapper) shutdown(ctx context.Context, data *shutdownData) error {
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

func newHTTPServer() *http.Server {
	return &http.Server{ //nolint:exhaustruct // server has a lot of fields
		// handler is 404 by default.
		Handler:           nil,
		ReadTimeout:       DefaultReadTimeout,
		ReadHeaderTimeout: DefaultReadHeaderTimeout,
		WriteTimeout:      DefaultWriteTimeout,
		IdleTimeout:       DefaultIdleTimeout,
	}
}

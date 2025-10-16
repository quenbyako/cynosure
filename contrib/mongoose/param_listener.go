package goose

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/url"

	"github.com/caarlos0/env/v11"
)

type netListenerWrapper struct {
	wrapped net.Listener

	network *url.URL
	config  *tls.Config
}

var _ net.Listener = (*netListenerWrapper)(nil)
var _ configureType = (*netListenerWrapper)(nil)

func parseListener(c *[]configureType) env.ParserFunc {
	return func(v string) (any, error) {
		u, err := url.Parse(v)
		if err != nil {
			return nil, fmt.Errorf("parsing listener URL %q: %w", v, err)
		}

		w := &netListenerWrapper{network: u}
		*c = append(*c, w)
		return w, nil
	}
}

func (l *netListenerWrapper) Accept() (net.Conn, error) { return l.wrapped.Accept() }
func (l *netListenerWrapper) Close() error              { return l.wrapped.Close() }
func (l *netListenerWrapper) Addr() net.Addr            { return l.wrapped.Addr() }
func (l *netListenerWrapper) configure(ctx context.Context, data *configureData) error {
	l.config = &tls.Config{
		MinVersion:               tls.VersionTLS12,
		Rand:                     nil,
		RootCAs:                  data.pool,
		ClientCAs:                data.pool,
		InsecureSkipVerify:       false,
		PreferServerCipherSuites: false,
		// TODO: any other options
	}

	return nil
}

func (l *netListenerWrapper) acquire(ctx context.Context, data *acquireData) (err error) {
	l.wrapped, err = net.Listen(l.network.Scheme, l.network.Host) // TODO: handle error
	if err != nil {
		return fmt.Errorf("listening on %q %q: %w", l.network.Scheme, l.network.Host, err)
	}

	if l.config != nil {
		l.wrapped = tls.NewListener(l.wrapped, l.config)
	}

	return nil
}

func (l *netListenerWrapper) shutdown(ctx context.Context, data *shutdownData) error {
	return l.wrapped.Close()
}

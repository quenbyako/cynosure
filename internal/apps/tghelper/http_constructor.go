package tghelper

import (
	"net/http"
	"time"
)

const (
	DefaultReadHeaderTimeout = 5 * time.Second
	DefaultReadTimeout       = 5 * time.Second
	DefaultWriteTimeout      = 10 * time.Second
	DefaultIdleTimeout       = 120 * time.Second

	DefaultServerStopTimeout = 5 * time.Second
)

func newHTTPServer(handler http.Handler) *http.Server {
	return &http.Server{ //nolint:exhaustruct // server has a lot of fields
		Handler:           handler,
		ReadTimeout:       DefaultReadTimeout,
		ReadHeaderTimeout: DefaultReadHeaderTimeout,
		WriteTimeout:      DefaultWriteTimeout,
		IdleTimeout:       DefaultIdleTimeout,
	}
}

package tghelper

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"tg-helper/contrib/onelog"

	"google.golang.org/grpc"
)

type App struct {
	log onelog.Logger

	httpAddr    net.Addr
	grpcAddr    net.Addr
	metricsAddr net.Addr

	grpcServer *grpc.Server
	httpServer *http.Server
}

func (a *App) StartOAuth(ctx context.Context) error {
	listener, err := net.Listen(a.httpAddr.Network(), a.httpAddr.String())
	if err != nil {
		return fmt.Errorf("starting to listen %q: %w", a.httpAddr.String(), err)
	}
	defer listener.Close()

	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		panic("unreachable")
	}

	a.log.Info().Str("host", host).Str("port", port).Msg("Starting oauth server")

	stopLocker := make(chan struct{})
	var shutdownErr error
	go func(err *error) {
		defer close(stopLocker)
		<-ctx.Done()

		timeoutCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		a.log.Info().Msg("Shutting down HTTP server...")
		*err = a.httpServer.Shutdown(timeoutCtx) //nolint:contextcheck // false positive
	}(&shutdownErr)

	err = a.httpServer.Serve(listener)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serving server: %w", err)
	}

	a.log.Info().Msg("HTTP server stopped")

	<-stopLocker

	return shutdownErr
}

func (a *App) StartGRPC(ctx context.Context) error {
	listener, err := net.Listen(a.grpcAddr.Network(), a.grpcAddr.String())
	if err != nil {
		return fmt.Errorf("starting to listen %q: %w", a.grpcAddr.String(), err)
	}
	defer listener.Close()

	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		panic("unreachable")
	}

	a.log.Info().Str("host", host).Str("port", port).Msg("Starting grpc server")

	stopLocker := make(chan struct{})
	go func() {
		defer close(stopLocker)
		<-ctx.Done()
		a.log.Info().Msg("Shutting down gRPC server...")
		a.grpcServer.GracefulStop()
	}()

	if err = a.grpcServer.Serve(listener); err != nil {
		a.log.Err(err).Msg("Server failed")

		return err //nolint:wrapcheck // no need to wrap
	}

	a.log.Info().Msg("gRPC server stopped")

	<-stopLocker

	return nil
}

func (a *App) StartMetrics(ctx context.Context) error {
	panic("StartMetrics not implemented")
}

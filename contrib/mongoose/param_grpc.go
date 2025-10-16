package goose

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strconv"

	"buf.build/go/protovalidate"
	"github.com/caarlos0/env/v11"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	validator "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/protovalidate"
	"github.com/quenbyako/cynosure/contrib/onelog"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/stats"

	"contrib/grpcutils"
)

type GRPCServer interface {
	grpc.ServiceRegistrar

	Serve(ctx context.Context) error
}

type grpcServerWrapper struct {
	log  onelog.Logger
	addr net.Addr

	conn net.Listener
	srv  *grpc.Server
}

var _ configureType = (*grpcServerWrapper)(nil)
var _ GRPCServer = (*grpcServerWrapper)(nil)

func parseGRPCServer(c *[]configureType) env.ParserFunc {
	return func(v string) (any, error) {
		u, err := url.Parse(v)
		if err != nil {
			return nil, err
		}
		if u.Scheme != "grpc" {
			return nil, fmt.Errorf("unsupported gRPC scheme %q", u.Scheme)
		}

		host := u.Hostname()
		ip := net.ParseIP(host)
		if ip == nil {
			return nil, fmt.Errorf("invalid gRPC host %q", host)
		}
		port := u.Port()
		if port == "" {
			return nil, fmt.Errorf("invalid gRPC port %q", port)
		}
		portNum, err := strconv.Atoi(port)
		if err != nil {
			return nil, fmt.Errorf("invalid gRPC port %q", port)
		}
		if portNum < 1 || portNum > 65535 {
			return nil, fmt.Errorf("out of range gRPC port %q", port)
		}
		addr := &net.TCPAddr{IP: ip, Port: portNum}

		w := &grpcServerWrapper{
			addr: addr,
		}
		*c = append(*c, w)
		return w, nil
	}
}

func (g *grpcServerWrapper) configure(ctx context.Context, data *configureData) error {
	g.srv = newGRPCServer(data.logger, data.metric, data.trace)
	g.log = onelog.Wrap(data.logger)

	return nil
}

func (g *grpcServerWrapper) acquire(ctx context.Context, data *acquireData) error {
	var err error
	g.conn, err = net.Listen(g.addr.Network(), g.addr.String()) // TODO: handle error correctly
	if err != nil {
		return fmt.Errorf("listening on %q %q: %w", g.addr.Network(), g.addr.String(), err)
	}

	return nil
}

func (g *grpcServerWrapper) RegisterService(sd *grpc.ServiceDesc, ss any) {
	g.srv.RegisterService(sd, ss)
}

func (g *grpcServerWrapper) Serve(ctx context.Context) error {
	stopLocker := make(chan struct{})
	go func() {
		defer close(stopLocker)
		<-ctx.Done()
		g.srv.GracefulStop()
	}()

	g.log.Info().Str("addr", g.addr.String()).Msg("starting gRPC server")

	if err := g.srv.Serve(g.conn); err != nil {
		return err //nolint:wrapcheck // no need to wrap
	}

	<-stopLocker

	g.log.Info().Str("addr", g.addr.String()).Msg("stopped gRPC server")

	return nil
}

func (g *grpcServerWrapper) shutdown(ctx context.Context, data *shutdownData) error {
	err := g.conn.Close()
	if err != nil {
		// poll.errNetClosing{}
		if err.Error() != "use of closed network connection" {
			return nil
		}

		return fmt.Errorf("closing connection: %w", err)
	}

	return nil
}

func newGRPCServer(logHandler slog.Handler, m metric.MeterProvider, t trace.TracerProvider) *grpc.Server {
	v := must(protovalidate.New())

	opts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(
			grpcutils.VerifyError(logHandler, nil),
			logging.UnaryServerInterceptor(
				interceptorLogger(logHandler),
				logging.WithLevels(defaultServerCodeToLevel),
			),
			validator.UnaryServerInterceptor(v),
		),
		grpc.ChainStreamInterceptor(
			logging.StreamServerInterceptor(
				interceptorLogger(logHandler),
				logging.WithLevels(defaultServerCodeToLevel),
			),
			validator.StreamServerInterceptor(v),
		),
		grpc.StatsHandler(grpcServerStats(m, t)),
	}

	srv := grpc.NewServer(opts...)

	// TODO(rcooper): make this optional
	reflection.Register(srv)

	return srv
}

// InterceptorLogger adapts slog logger to interceptor logger.
// This code is simple enough to be copied and not imported.
func interceptorLogger(l slog.Handler) logging.LoggerFunc {
	logger := slog.New(l)

	return logging.LoggerFunc(func(ctx context.Context, lvl logging.Level, msg string, fields ...any) {
		logger.Log(ctx, slog.Level(lvl), msg, fields...)
	})
}

func grpcServerStats(m metric.MeterProvider, t trace.TracerProvider) stats.Handler {
	return otelgrpc.NewServerHandler(
		otelgrpc.WithMeterProvider(m),
		otelgrpc.WithTracerProvider(t),
	)
}

func grpcClientStats(m metric.MeterProvider, t trace.TracerProvider) stats.Handler {
	return otelgrpc.NewClientHandler(
		otelgrpc.WithMeterProvider(m),
		otelgrpc.WithTracerProvider(t),
	)
}

func defaultServerCodeToLevel(code codes.Code) logging.Level {
	switch code {
	case
		codes.OK,
		codes.NotFound,
		codes.Canceled,
		codes.AlreadyExists,
		codes.InvalidArgument,
		codes.Unauthenticated:
		return logging.LevelDebug

	case
		codes.DeadlineExceeded,
		codes.PermissionDenied,
		codes.ResourceExhausted,
		codes.FailedPrecondition,
		codes.Aborted,
		codes.OutOfRange,
		codes.Unavailable,
		codes.Unknown,
		codes.Unimplemented,
		codes.Internal,
		codes.DataLoss:
		return logging.LevelWarn
	default:
		return logging.LevelError
	}
}

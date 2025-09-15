package tghelper

import (
	"context"
	"log/slog"
	"tg-helper/internal/apps/tghelper/metrics"

	"contrib/grpcutils"

	"buf.build/go/protovalidate"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	validator "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/protovalidate"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/stats"
)

func newGRPCServer(logHandler slog.Handler, m metrics.Metrics, registars ...func(reflection.GRPCServer)) *grpc.Server {
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
		grpc.StatsHandler(grpcServerStats(m)),
	}

	server := grpc.NewServer(opts...)

	for _, register := range registars {
		register(server)
	}

	return server
}

// InterceptorLogger adapts slog logger to interceptor logger.
// This code is simple enough to be copied and not imported.
func interceptorLogger(l slog.Handler) logging.LoggerFunc {
	logger := slog.New(l)

	return logging.LoggerFunc(func(ctx context.Context, lvl logging.Level, msg string, fields ...any) {
		logger.Log(ctx, slog.Level(lvl), msg, fields...)
	})
}

func grpcServerStats(m metrics.Metrics) stats.Handler {
	return otelgrpc.NewServerHandler(
		otelgrpc.WithMeterProvider(m),
		otelgrpc.WithTracerProvider(m),
	)
}

func grpcClientStats(m metrics.Metrics) stats.Handler {
	return otelgrpc.NewClientHandler(
		otelgrpc.WithMeterProvider(m),
		otelgrpc.WithTracerProvider(m),
	)
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

const traceLogLevel logging.Level = logging.LevelDebug - 4

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

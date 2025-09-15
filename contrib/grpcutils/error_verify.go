package grpcutils

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// VerifyError returns a gRPC interceptor that checks that implementation
// returns correct and detailed error messages.
func VerifyError(log slog.Handler, now func() time.Time) grpc.UnaryServerInterceptor {
	if now == nil {
		now = time.Now
	}
	if log == nil {
		panic("log handler cannot be nil")
	}

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		resp, err = handler(ctx, req)

		var record *slog.Record

		if status, ok := status.FromError(err); !ok {
			record = &slog.Record{
				Time:    now(),
				Level:   slog.LevelWarn,
				Message: "Handler tries to send non gRPC error",
			}
			record.AddAttrs(
				slog.Any("error", err),
				slog.String("grpc.method", info.FullMethod),
			)
		} else if code := status.Code(); code < codes.OK || code > codes.Unauthenticated {
			record = &slog.Record{
				Time:    now(),
				Level:   slog.LevelWarn,
				Message: "Handler tries to send invalid status code",
			}
			record.AddAttrs(
				slog.Any("error", err),
				slog.String("grpc.method", info.FullMethod),
				slog.String("grpc.code", code.String()),
			)
		} else if validateErr := validateDetails(status); validateErr != nil {
			record = &slog.Record{
				Time:    now(),
				Level:   slog.LevelWarn,
				Message: "Handler returned status with invalid details",
			}
			record.AddAttrs(
				slog.Any("error", err),
				slog.String("grpc.method", info.FullMethod),
			)
		}

		if record != nil {
			log.Handle(ctx, *record)
		}

		return resp, err
	}
}

func validateDetails(s *status.Status) error {
	if s == nil {
		return nil
	}

	var errs []error

	var hasDetails bool

	types := make(map[string]struct{}, len(s.Details()))

	for _, detail := range s.Details() {
		if detail == nil {
			errs = append(errs, errors.New("status contains nil detail"))
			continue
		}

		if _, ok := detail.(*errdetails.ErrorInfo); ok {
			hasDetails = true
		}

		typeStr := fmt.Sprintf("%T", detail)
		if _, ok := types[typeStr]; ok {
			errs = append(errs, fmt.Errorf("status contains duplicate detail type %s", typeStr))
			continue
		}

		types[typeStr] = struct{}{}
	}

	if !hasDetails {
		errs = append(errs, errors.New("status does not contain ErrorInfo"))
	}

	switch len(errs) {
	case 0:
		return nil
	case 1:
		return errs[0]
	default:
		return errors.Join(errs...)
	}
}

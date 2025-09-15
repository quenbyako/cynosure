package grpcutils

import (
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// using alias to avoid adding this type to types table in compiled binary
type grpcstatus = interface {
	error
	GRPCStatus() *status.Status
}

type InvalidArgumentError struct {
	Violations []*errdetails.BadRequest_FieldViolation
}

var _ grpcstatus = (*InvalidArgumentError)(nil) //nolint:errcheck // WHAT???

func ErrInvalidArgument(violations ...*errdetails.BadRequest_FieldViolation) *InvalidArgumentError {
	filtered := make([]*errdetails.BadRequest_FieldViolation, 0, len(violations))

	for _, v := range violations {
		if v != nil {
			filtered = append(filtered, v)
		}
	}

	return &InvalidArgumentError{
		Violations: filtered,
	}
}

func (e *InvalidArgumentError) Error() string {
	if len(e.Violations) == 0 {
		return "invalid argument"
	}

	msg := "invalid argument: "

	for i, v := range e.Violations {
		if i > 0 {
			msg += ", "
		}

		msg += v.GetField() + ": " + v.GetDescription()
	}

	return msg
}

func (e *InvalidArgumentError) GRPCStatus() *status.Status {
	s := status.New(codes.InvalidArgument, e.Error())
	if len(e.Violations) == 0 {
		return s
	}

	return must(s.WithDetails(
		&errdetails.ErrorInfo{
			Reason:   "REASON_INVALID_ARGUMENT",
			Domain:   "",
			Metadata: nil,
		},
		&errdetails.BadRequest{
			FieldViolations: e.Violations,
		},
	))
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err) //nolint:forbidigo // there is a purpose for this...
	}

	return v
}

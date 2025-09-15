package grpcutils

import (
	"errors"
	"strings"

	"buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
	"buf.build/go/protovalidate"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func Validate(validator protovalidate.Validator, msg protoreflect.ProtoMessage) error {
	err := validator.Validate(msg)
	if err == nil {
		return nil
	}

	var violations []*errdetails.BadRequest_FieldViolation
	if e := new(protovalidate.ValidationError); errors.As(err, &e) {
		violations = make([]*errdetails.BadRequest_FieldViolation, len(e.Violations))
		// using proto representaition, cause i have no idea how to conver
		// protoreflect.Descriptor into path.
		for i, v := range e.ToProto().GetViolations() {
			violations[i] = violationToProto(v)
		}
	}

	return ErrInvalidArgument(violations...)
}

func violationToProto(violation *validate.Violation) *errdetails.BadRequest_FieldViolation {
	if violation == nil {
		return nil
	}

	elements := violation.GetField().GetElements()
	path := make([]string, len(elements))

	for i, f := range elements {
		if f == nil {
			path[i] = "???"
		} else {
			path[i] = f.GetFieldName()
		}
	}

	var msg string
	if violation.Message != nil {
		msg = violation.GetMessage()
	} else {
		msg = violation.String() // todo: how to make it better?
	}

	return &errdetails.BadRequest_FieldViolation{
		Field:            strings.Join(path, "."),
		Reason:           violation.GetRuleId(),
		Description:      msg,
		LocalizedMessage: nil,
	}
}

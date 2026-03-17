package telegram

import (
	"errors"
	"fmt"
)

var ErrInvalidMessageID = errors.New("invalid message id")

type APIError struct {
	op     string
	status string
}

func errAPI(op, status string) *APIError {
	return &APIError{op: op, status: status}
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%v: %q", e.op, e.status)
}

func (e *APIError) Op() string     { return e.op }
func (e *APIError) Status() string { return e.status }

type InternalValidationError string

func (e InternalValidationError) Error() string { return string(e) }

func ErrInternalValidation(format string, a ...any) error {
	return InternalValidationError(fmt.Sprintf(format, a...))
}

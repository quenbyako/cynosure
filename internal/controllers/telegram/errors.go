package telegram

import (
	"errors"
	"fmt"

	"github.com/quenbyako/cynosure/contrib/tg-openapi/gen/go/botapi"
)

var ErrInvalidMessageID = errors.New("invalid message id")

type APIError struct {
	op          string
	status      string
	description string
}

func errAPI(operation, status string, err *botapi.Error) *APIError {
	if err != nil {
		return &APIError{
			op:          operation,
			status:      status,
			description: err.Description,
		}
	}

	return &APIError{
		op:          operation,
		status:      status,
		description: "",
	}
}

func (e *APIError) Error() string {
	if e.description != "" {
		return fmt.Sprintf("%v, %q: %q", e.op, e.status, e.description)
	}

	return fmt.Sprintf("%v, %q", e.op, e.status)
}

func (e *APIError) Op() string          { return e.op }
func (e *APIError) Status() string      { return e.status }
func (e *APIError) Description() string { return e.description }

type InternalValidationError string

func (e InternalValidationError) Error() string { return string(e) }

func ErrInternalValidation(format string, a ...any) error {
	return InternalValidationError(fmt.Sprintf(format, a...))
}

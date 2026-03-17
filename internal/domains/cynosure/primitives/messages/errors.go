package messages

import (
	"errors"
	"fmt"
)

var (
	ErrMessageTooLarge     = errors.New("message is too large")
	ErrToolMessageNoFormat = errors.New("tool message cannot be formatted")
)

type InternalValidationError string

func (e InternalValidationError) Error() string {
	return string(e)
}

func ErrInternalValidation(format string, a ...any) error {
	return InternalValidationError(fmt.Sprintf(format, a...))
}

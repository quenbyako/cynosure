package accounts

import (
	"errors"
	"fmt"
)

var (
	ErrUserNotFound          = errors.New("user does not exist")
	ErrAccountIDAlreadySet   = errors.New("account ID is already set")
	ErrStateRequired         = errors.New("state parameter is required")
	ErrExchangeTokenRequired = errors.New("exchange token is required")
)

type InternalValidationError string

func (e InternalValidationError) Error() string {
	return string(e)
}

func ErrInternalValidation(format string, a ...any) error {
	return InternalValidationError(fmt.Sprintf(format, a...))
}

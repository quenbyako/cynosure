package accounts

import (
	"errors"
	"fmt"
)

var (
	ErrUserNotFound            = errors.New("user does not exist")
	ErrAccountIDAlreadySet     = errors.New("account ID is already set")
	ErrStateRequired           = errors.New("state parameter is required")
	ErrExchangeTokenRequired   = errors.New("exchange token is required")
	ErrAccessDenied            = errors.New("access denied")
	ErrUnknownSearchResultType = errors.New("unknown account search result type")
	ErrAuthUnsupported         = errors.New(
		"authorization for this server is not supported, allowed to connect anonymously",
	)
)

type InternalValidationError string

func (e InternalValidationError) Error() string {
	return string(e)
}

func ErrInternalValidation(format string, a ...any) error {
	return InternalValidationError(fmt.Sprintf(format, a...))
}

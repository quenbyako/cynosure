package types

import (
	"errors"
	"fmt"
)

var (
	ErrNilObject = errors.New("nil value is not allowed")
)

type InvalidEnumError struct {
	Value string
}

var _ error = (*InvalidEnumError)(nil)

func ErrInvalidEnum(value string) *InvalidEnumError {
	return &InvalidEnumError{Value: value}
}

func (e *InvalidEnumError) Error() string {
	return fmt.Sprintf("value %q is not valid", string(e.Value))
}

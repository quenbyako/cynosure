package httpclient

import (
	"errors"
	"fmt"
)

var (
	errUnconfigured = errors.New("http client not configured")
)

type UnsupportedSchemeError struct {
	Scheme string
}

func (e *UnsupportedSchemeError) Error() string {
	return fmt.Sprintf("unsupported http client scheme %q", e.Scheme)
}

func ErrUnsupportedScheme(scheme string) error {
	return &UnsupportedSchemeError{Scheme: scheme}
}

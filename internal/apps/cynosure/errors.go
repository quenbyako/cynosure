package cynosure

import (
	"errors"
)

type MissingParamError string

func (e MissingParamError) Error() string {
	return "missing " + string(e)
}

var (
	ErrNoSuitableAdapter             = errors.New("no suitable adapter found")
	ErrAmbiguousAdapter              = errors.New("ambiguous adapter configuration")
	ErrPreferredAdapterNotConfigured = errors.New(
		"preferred adapter is explicitly selected but not configured",
	)
)

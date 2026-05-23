package cynosure

import (
	"errors"
)

type MissingParamError string

func (e MissingParamError) Error() string {
	return "missing " + string(e)
}

var (
	ErrNoSuitableAdapter  = errors.New("no suitable adapter found")
	ErrAmbiguousAdapter   = errors.New("ambiguous adapter configuration")
	ErrKeysMissing        = errors.New("at least one of Gemini or OpenRouter keys must be provided")
	ErrInvalidCacheScheme = errors.New("openrouter cache dir scheme must be 'file'")

	ErrPreferredAdapterNotConfigured = errors.New(
		"preferred adapter is explicitly selected but not configured",
	)
)

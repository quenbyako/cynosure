package ports

import (
	"errors"
)

var (
	ErrNotFound = errors.New("not found")

	ErrToolsNotCached = errors.New("tools were not cached")

	ErrAuthUnsupported = errors.New("authorization for this server is not supported, allowed to connect anonymously")
)

type RequiresAuthError struct {
	Endpoint     string
	State        string
	CodeVerifier string
}

var _ error = (*RequiresAuthError)(nil)

func (e *RequiresAuthError) Error() string {
	return "requires authentication at: " + e.Endpoint
}

func ErrRequiresAuth(endpoint, state, codeVerifier string) *RequiresAuthError {
	return &RequiresAuthError{Endpoint: endpoint, State: state, CodeVerifier: codeVerifier}
}

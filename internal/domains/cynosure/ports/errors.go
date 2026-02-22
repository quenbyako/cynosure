package ports

import (
	"errors"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")

	ErrToolsNotCached = errors.New("tools were not cached")

	ErrAuthUnsupported = errors.New("authorization for this server is not supported, allowed to connect anonymously")

	// ErrAuthRequired indicates that authentication is required but was not
	// provided. Use case should prompt user to authenticate or provide OAuth
	// token.
	ErrAuthRequired = errors.New("authentication required")

	// ErrServerUnreachable indicates that all connection protocols failed.
	// Use case should inform user that server is offline or unreachable.
	ErrServerUnreachable = errors.New("server unreachable")

	// ErrInvalidCredentials indicates that provided OAuth token was rejected.
	// Use case should prompt user to re-authenticate.
	ErrInvalidCredentials = errors.New("invalid credentials")

	// ErrProtocolNotSupported indicates that server doesn't support any known
	// protocols.
	ErrProtocolNotSupported = errors.New("protocol not supported")
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

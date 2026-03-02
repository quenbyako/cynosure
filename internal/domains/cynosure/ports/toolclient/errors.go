package toolclient

import (
	"errors"
	"fmt"
	"net/url"
)

var (
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
	suggestedMetadataEndpoint *url.URL
}

var _ error = (*RequiresAuthError)(nil)

func ErrRequiresAuth(metadataEndpoint *url.URL) *RequiresAuthError {
	return &RequiresAuthError{
		suggestedMetadataEndpoint: metadataEndpoint,
	}
}

func (e *RequiresAuthError) Error() string {
	if e.suggestedMetadataEndpoint == nil {
		return "requires authentication, no metadata endpoint suggested"
	}
	return fmt.Sprintf("requires authentication, should use metadata endpoint: %s", e.suggestedMetadataEndpoint.String())
}

func (e *RequiresAuthError) Endpoint() *url.URL { return e.suggestedMetadataEndpoint }

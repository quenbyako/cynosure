package oauthhandler

import (
	"errors"
	"net/url"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")

	ErrToolsNotCached = errors.New("tools were not cached")

	ErrAuthUnsupported = errors.New("authorization for this server is not supported, allowed to connect anonymously")

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

	return "requires authentication, should use metadata endpoint: " + e.suggestedMetadataEndpoint.String()
}

func (e *RequiresAuthError) Endpoint() *url.URL { return e.suggestedMetadataEndpoint }

type DynamicClientRegistrationNotSupportedError struct {
	resourceDocumentationEndpoint *url.URL
}

var _ error = (*DynamicClientRegistrationNotSupportedError)(nil)

func ErrDynamicClientRegistrationNotSupported(resourceDocumentationEndpoint *url.URL) *DynamicClientRegistrationNotSupportedError {
	return &DynamicClientRegistrationNotSupportedError{
		resourceDocumentationEndpoint: resourceDocumentationEndpoint,
	}
}

func (e *DynamicClientRegistrationNotSupportedError) Error() string {
	if e.resourceDocumentationEndpoint == nil {
		return "server does not support dynamic client registration"
	}

	return "server does not support dynamic client registration. Check resource documentation: " + e.resourceDocumentationEndpoint.String()
}

func (e *DynamicClientRegistrationNotSupportedError) Documentation() *url.URL {
	return e.resourceDocumentationEndpoint
}

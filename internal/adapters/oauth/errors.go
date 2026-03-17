package oauth

import (
	"fmt"
)

// OAuthError represents a standard OAuth 2.0 error response
type OAuthError struct {
	ErrorCode        string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
	ErrorURI         string `json:"error_uri,omitempty"`
}

// Error implements the error interface
func (e OAuthError) Error() string {
	if e.ErrorDescription != "" {
		return fmt.Sprintf("OAuth error: %s - %s", e.ErrorCode, e.ErrorDescription)
	}

	return "OAuth error: " + e.ErrorCode
}

// RegistrationError is returned when server responds with unexpected status
// during client registration. It is an interface error that callers can inspect.
type RegistrationError struct {
	Endpoint   string
	Body       string
	StatusCode int
}

func (e *RegistrationError) Error() string {
	msg := fmt.Sprintf(
		"unexpected status code %d when registering client at %s",
		e.StatusCode, e.Endpoint,
	)
	if e.Body != "" {
		return msg + ": " + e.Body
	}

	return msg
}

// InternalValidationError is a non-handleable validation error for developer
// mistakes (nil URLs, empty required fields, etc).
type InternalValidationError string

func (e InternalValidationError) Error() string { return string(e) }

func errInternalValidation(format string, a ...any) error {
	return InternalValidationError(fmt.Sprintf(format, a...))
}

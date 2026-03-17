package chat

import (
	"errors"
	"fmt"
)

var (
	// ErrNoAgentsFound is returned when no agents are available for a user.
	ErrNoAgentsFound = errors.New("no agents found")

	// ErrUnexpectedMessageType is returned when an unknown message type is encountered.
	ErrUnexpectedMessageType = errors.New("unexpected message type")
)

// InternalValidationError is returned when usecase configuration or parameters are invalid.
type InternalValidationError struct {
	Message string
}

func (e *InternalValidationError) Error() string {
	return "chat usecase validation error: " + e.Message
}

// errInternalValidation is a helper to create InternalValidationError.
func errInternalValidation(msg string, args ...any) error {
	return &InternalValidationError{Message: fmt.Sprintf(msg, args...)}
}

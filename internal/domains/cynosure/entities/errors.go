package entities

import (
	"fmt"
)

// InternalValidationError is a non-handleable validation error meant to catch
// developer mistakes at construction time (e.g. invalid IDs, missing required
// fields). Clients MUST NOT rely on the specific message text.
type InternalValidationError string

func (e InternalValidationError) Error() string { return string(e) }

// ErrInternalValidation creates an InternalValidationError with a formatted message.
func ErrInternalValidation(format string, a ...any) error {
	return InternalValidationError(fmt.Sprintf(format, a...))
}

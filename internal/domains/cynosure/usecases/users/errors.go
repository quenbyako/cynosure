package users

// InternalValidationError is returned when usecase configuration or parameters are invalid.
type InternalValidationError struct {
	Message string
}

func (e *InternalValidationError) Error() string {
	return "users usecase validation error: " + e.Message
}

// errInternalValidation is a helper to create InternalValidationError.
func errInternalValidation(msg string) error {
	return &InternalValidationError{Message: msg}
}

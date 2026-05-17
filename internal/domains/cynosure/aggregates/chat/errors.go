package chat

import (
	"fmt"
)

type InternalValidationError string

func (e InternalValidationError) Error() string {
	return string(e)
}

func errInternalValidation(format string) error {
	return InternalValidationError(format)
}

type ToolIDNotPendingError struct {
	toolID string
}

func (e *ToolIDNotPendingError) Error() string {
	return fmt.Sprintf("unexpected tool result: ID %q is not pending in the current turn", e.toolID)
}

func (e *ToolIDNotPendingError) ToolID() string { return e.toolID }

func errToolIDNotPending(toolID string) *ToolIDNotPendingError {
	return &ToolIDNotPendingError{toolID: toolID}
}

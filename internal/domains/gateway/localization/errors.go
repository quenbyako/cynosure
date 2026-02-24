package localization

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// ErrAgentTimeout is returned when the agent takes too long to respond.
	ErrAgentTimeout = "⏱ The agent is taking too long to respond. Please try again later."

	// ErrServiceUnavailable is returned when the agent service is down.
	ErrServiceUnavailable = "🔌 The agent service is temporarily unavailable. Please try again in a few moments."

	// ErrRequestCanceled is returned when the request was canceled.
	ErrRequestCanceled = "🚫 The request was canceled. Please try again."

	// ErrServiceOverloaded is returned when the service is under heavy load.
	ErrServiceOverloaded = "⚠️ The service is currently overloaded. Please try again in a few moments."

	// ErrAuthenticationFailed is returned when credentials are invalid.
	ErrAuthenticationFailed = "🔐 Authentication failed. Please check your credentials."

	// ErrPermissionDenied is returned when the user lacks permission.
	ErrPermissionDenied = "🚫 You don't have permission to perform this action."

	// ErrInvalidFormat is returned when the input message is invalid.
	ErrInvalidFormat = "❌ Invalid message format. Please check your input."

	// ErrUnexpected is a generic error message with details.
	ErrUnexpected = "❌ An unexpected error occurred: %v"
)

// UserFriendlyError converts technical errors into user-friendly messages with emojis.
// This function categorizes common failure scenarios and provides helpful guidance to users.
func UserFriendlyError(err error) string {
	if err == nil {
		return ""
	}

	// Check for context deadline exceeded
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrAgentTimeout
	}

	// Check for gRPC status errors
	if st, ok := status.FromError(err); ok {
		switch st.Code() {
		case codes.Unavailable:
			return ErrServiceUnavailable
		case codes.DeadlineExceeded:
			return ErrAgentTimeout
		case codes.Canceled:
			return ErrRequestCanceled
		case codes.ResourceExhausted:
			return ErrServiceOverloaded
		case codes.Unauthenticated:
			return ErrAuthenticationFailed
		case codes.PermissionDenied:
			return ErrPermissionDenied
		case codes.InvalidArgument:
			return ErrInvalidFormat
		}
	}

	// Default error message
	return fmt.Sprintf(ErrUnexpected, err)
}

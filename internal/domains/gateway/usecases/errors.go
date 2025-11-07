package usecases

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// T009: Error categorization helper for user-friendly error messages
func userFriendlyError(err error) string {
	if err == nil {
		return ""
	}

	// Check for context deadline exceeded
	if errors.Is(err, context.DeadlineExceeded) {
		return "⏱ The agent is taking too long to respond. Please try again later."
	}

	// Check for gRPC status errors
	if st, ok := status.FromError(err); ok {
		switch st.Code() {
		case codes.Unavailable:
			return "🔌 The agent service is temporarily unavailable. Please try again in a few moments."
		case codes.DeadlineExceeded:
			return "⏱ The agent is taking too long to respond. Please try again later."
		case codes.Canceled:
			return "🚫 The request was canceled. Please try again."
		case codes.ResourceExhausted:
			return "⚠️ The service is currently overloaded. Please try again in a few moments."
		case codes.Unauthenticated:
			return "🔐 Authentication failed. Please check your credentials."
		case codes.PermissionDenied:
			return "🚫 You don't have permission to perform this action."
		case codes.InvalidArgument:
			return "❌ Invalid message format. Please check your input."
		}
	}

	// Default error message
	return fmt.Sprintf("❌ An unexpected error occurred: %v", err)
}

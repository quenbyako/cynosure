package localization

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestUserFriendlyError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: "",
		},
		{
			name:     "context deadline exceeded",
			err:      context.DeadlineExceeded,
			expected: ErrAgentTimeout,
		},
		{
			name:     "grpc unavailable",
			err:      status.Error(codes.Unavailable, "service unavailable"),
			expected: ErrServiceUnavailable,
		},
		{
			name:     "grpc deadline exceeded",
			err:      status.Error(codes.DeadlineExceeded, "deadline exceeded"),
			expected: ErrAgentTimeout,
		},
		{
			name:     "grpc canceled",
			err:      status.Error(codes.Canceled, "canceled"),
			expected: ErrRequestCanceled,
		},
		{
			name:     "grpc resource exhausted",
			err:      status.Error(codes.ResourceExhausted, "resource exhausted"),
			expected: ErrServiceOverloaded,
		},
		{
			name:     "grpc unauthenticated",
			err:      status.Error(codes.Unauthenticated, "unauthenticated"),
			expected: ErrAuthenticationFailed,
		},
		{
			name:     "grpc permission denied",
			err:      status.Error(codes.PermissionDenied, "permission denied"),
			expected: ErrPermissionDenied,
		},
		{
			name:     "grpc invalid argument",
			err:      status.Error(codes.InvalidArgument, "invalid argument"),
			expected: ErrInvalidFormat,
		},
		{
			name:     "unexpected error",
			err:      errors.New("something went wrong"),
			expected: fmt.Sprintf(ErrUnexpected, errors.New("something went wrong")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := UserFriendlyError(tt.err)
			if actual != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, actual)
			}
		})
	}
}

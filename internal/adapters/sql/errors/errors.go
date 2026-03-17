// Package errors defines errors for SQL adapter.
package errors

import (
	"errors"
)

var (
	// ErrEmptyResultSet is returned when no rows are found.
	ErrEmptyResultSet = errors.New("empty result set")

	// ErrMessageTypeNil is returned when a message has no type.
	ErrMessageTypeNil = errors.New("message type is nil")

	// ErrMessageTypeUnknown is returned when an unknown message type is encountered.
	ErrMessageTypeUnknown = errors.New("unknown message type")

	// ErrUserContentNil is returned when a user message has no content.
	ErrUserContentNil = errors.New("user content is nil")

	// ErrToolRequestFieldsMissing is returned when tool request fields are missing.
	ErrToolRequestFieldsMissing = errors.New("tool request fields missing")

	// ErrToolResultContentMissing is returned when tool result content is missing.
	ErrToolResultContentMissing = errors.New("tool result content missing")

	// ErrToolResultCallIDMissing is returned when tool result call ID is missing.
	ErrToolResultCallIDMissing = errors.New("tool result call id missing")

	// ErrToolResultOriginMissing is returned when the origin request for a tool result is missing.
	ErrToolResultOriginMissing = errors.New("tool result origin request missing")

	// ErrThreadNoMessages is returned when a thread has no messages in database.
	ErrThreadNoMessages = errors.New("thread has no messages in database")

	// ErrOAuthAuthURLEmpty is returned when the OAuth auth URL is empty.
	ErrOAuthAuthURLEmpty = errors.New("invalid oauth config: auth URL is empty")

	// ErrOAuthTokenURLEmpty is returned when the OAuth token URL is empty.
	ErrOAuthTokenURLEmpty = errors.New("invalid oauth config: token URL is empty")

	// ErrConcurrentModification is returned when a concurrent modification is detected.
	ErrConcurrentModification = errors.New("concurrent modification")

	// ErrPoolNil is returned when pgxpool is nil.
	ErrPoolNil = errors.New("pool is nil")

	// ErrTraceNil is returned when tracer is nil.
	ErrTraceNil = errors.New("trace is nil")

	// ErrTracerProviderNil is returned when tracer provider is nil.
	ErrTracerProviderNil = errors.New("tracer provider is nil")
)

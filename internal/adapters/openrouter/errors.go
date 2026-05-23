package openrouter

import (
	"errors"
)

var (
	ErrUnknownToolChoice          = errors.New("unknown tool choice")
	ErrPreflightFailed            = errors.New("preflight check failed")
	ErrHardQuotaExhausted         = errors.New("hard quota exhausted")
	ErrInternalValidation         = errors.New("internal validation failed")
	ErrEmptyEmbeddingData         = errors.New("empty data in embedding response")
	ErrEmbeddingDimensionMismatch = errors.New("embedding dimension mismatch")
	ErrRequestFailed              = errors.New("openrouter request failed")
	ErrTokenCountOutOfBounds      = errors.New("token count out of bounds")

	// ErrRetryableStatus is returned when the server responds with a retryable
	// HTTP status code.
	ErrRetryableStatus = errors.New("retryable status code")
)

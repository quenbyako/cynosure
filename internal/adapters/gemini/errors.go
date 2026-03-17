package gemini

import (
	"errors"
	"fmt"
)

var (
	ErrUnknownToolChoice = errors.New("unknown tool choice")
	ErrNoEmbeddings      = errors.New("no embeddings returned")
	ErrTracerProviderNil = errors.New("tracer provider is nil")
)

type EmbeddingDimensionError struct {
	Got  int
	Want int
}

func ErrEmbeddingDimension(got, want int) *EmbeddingDimensionError {
	return &EmbeddingDimensionError{Got: got, Want: want}
}

func (e *EmbeddingDimensionError) Error() string {
	return fmt.Sprintf("unexpected embedding dimension: got %d, want %d", e.Got, e.Want)
}

type InternalValidationError string

func (e InternalValidationError) Error() string {
	return string(e)
}

func ErrInternalValidation(format string, a ...any) error {
	return InternalValidationError(fmt.Sprintf(format, a...))
}

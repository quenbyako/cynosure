package rfc9110

import (
	"errors"
)

var (
	// ErrUnexpectedItem is returned when an item type is unexpected during
	// collection.
	ErrUnexpectedItem = errors.New("unexpected item type")

	// ErrMissedOpeningQuote is returned when a quoted string is missing its
	// opening quote.
	ErrMissedOpeningQuote = errors.New("missed opening quote")

	// ErrUnclosedQuote is returned when a quoted string is missing its closing
	// quote.
	ErrUnclosedQuote = errors.New("unclosed quote")

	// ErrUnexpectedEOF is returned when the end of the input is reached
	// unexpectedly.
	ErrUnexpectedEOF = errors.New("unexpected EOF")
)

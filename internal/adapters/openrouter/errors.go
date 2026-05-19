package openrouter

import (
	"errors"
)

var (
	ErrUnknownToolChoice  = errors.New("unknown tool choice")
	ErrPreflightFailed    = errors.New("preflight check failed")
	ErrHardQuotaExhausted = errors.New("hard quota exhausted")
	ErrInternalValidation = errors.New("internal validation failed")
)

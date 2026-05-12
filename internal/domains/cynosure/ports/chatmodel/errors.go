package chatmodel

import (
	"errors"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

var (
	ErrHistoryTooLong     = messages.ErrInternalValidation("history is too long")
	ErrHardQuotaExhausted = errors.New("hard quota exhausted")
)

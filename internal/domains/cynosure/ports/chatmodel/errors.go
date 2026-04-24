package chatmodel

import (
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

var ErrHistoryTooLong = messages.ErrInternalValidation("history is too long")

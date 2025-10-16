package primitive

import (
	"context"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/messages"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/tools"
)

// RetrieveRelevantTools implements ports.ToolManager.
func (h *Handler) RetrieveRelevantTools(ctx context.Context, user ids.UserID, input []messages.Message) (map[ids.AccountID][]tools.ToolInfo, error) {
	result := make(map[ids.AccountID][]tools.ToolInfo)
	accounts, err := h.accounts.ListAccounts(ctx, user)
	if err != nil {
		return nil, err
	}

	for _, acc := range accounts {
		session, err := h.accounts.GetAccount(ctx, acc)
		if err != nil {
			return nil, err
		}

		result[acc] = session.Tools()
	}

	return result, nil
}

package ports

import (
	"context"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/tools"
)

type ToolCache interface {
	SetTools(ctx context.Context, account ids.AccountID, toolSet []tools.ToolInfo) error
	GetTools(ctx context.Context, account ids.AccountID) ([]tools.ToolInfo, error)
}

type ToolCacheFactory interface {
	ToolCache() ToolCache
}

func NewToolCache(factory ToolCacheFactory) ToolCache { return factory.ToolCache() }

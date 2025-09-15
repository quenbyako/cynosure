package ports

import (
	"context"

	"tg-helper/internal/domains/components/ids"
	"tg-helper/internal/domains/components/messages"
	"tg-helper/internal/domains/components/tools"
)

type ToolCache interface {
	SetTools(ctx context.Context, account ids.AccountID, toolSet []tools.ToolInfo) error
	GetTools(ctx context.Context, account ids.AccountID) ([]tools.ToolInfo, error)
}

type MCPClient interface {
	Capabilities(ctx context.Context) ([]tools.ToolInfo, error)
	Execute(ctx context.Context, call tools.ToolCall) (messages.MessageTool, error)
}

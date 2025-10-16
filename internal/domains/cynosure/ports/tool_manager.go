package ports

import (
	"context"

	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/messages"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/tools"
)

// Note: AccountID already contains user id, and server id, so implementation
// already knows where to connect and who connects.

type ToolManager interface {
	// RegisterTools registers a new tool in the tool manager. If
	//
	// accountID defines the name of the account that owns the tool, it will be
	// used to associate the tool with the account. Tool manager should not
	// manage access to those tools, instead, it's single responsibility is to
	// register tools and provide access to them.
	//
	// Method is idempotent, if tool with same account exists, it will return
	// same account id.
	//
	// if method receives no token, and wonn't catch authorization error during
	// initial connection, it will behave as public tool, meaning that same
	// connection can be reused by different users at the same time.
	//
	// Method should be called right after new account was created,
	// asynchronously of user's messages. It is also recommended to show on
	// user's dashboard status of tool registration.
	//
	// Should return [*RequiresAuthError], if tool requires authentication.
	RegisterTools(ctx context.Context, account ids.AccountID, name, description string, token *oauth2.Token) error

	// RetrieveRelevantTools retrieves a list of tools relevant to the user's input.
	RetrieveRelevantTools(ctx context.Context, user ids.UserID, input []messages.Message) (map[ids.AccountID][]tools.ToolInfo, error)

	// ExecuteTool executes a tool call on the tool set.
	//
	// ToolCall already contains account id, so implementation
	// already knows which connection it should use
	ExecuteTool(ctx context.Context, call tools.ToolCall) (messages.MessageTool, error)
}

type ToolManagerFactory interface {
	ToolManager() ToolManager
}

func NewToolManager(factory ToolManagerFactory) ToolManager { return factory.ToolManager() }

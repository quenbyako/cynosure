package ports

import (
	"context"
	"encoding/json"
	"net/url"

	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
)

// ToolClient executes MCP (Model Context Protocol) operations: tool discovery
// and tool execution. Abstracts MCP server connections, protocol handling, and
// account-based access control.
type ToolClient interface {
	// DiscoverTools retrieves available tools from an MCP server. Implements
	// the MCP tool discovery phase, returning tools with their schemas.
	//
	// See next test suites to find how it works:
	//
	//  - [TestDiscoverTools] — discovering tools from MCP servers
	//
	// Throws:
	//
	//  - [ErrServerUnreachable] if server connection fails.
	//  - [ErrInvalidCredentials] if OAuth token is invalid or expired.
	DiscoverTools(ctx context.Context, u *url.URL, token *oauth2.Token, account ids.AccountID, accountDesc string) ([]tools.RawToolInfo, error)

	// ExecuteTool executes a tool call and returns the result. Implements the
	// MCP tool execution phase. Does not validate argument schemas - validation
	// happens in domain layer.
	//
	// See next test suites to find how it works:
	//
	//  - [TestExecuteTool] — executing tool calls and handling results
	//
	// Throws:
	//
	//  - [ErrServerUnreachable] if server connection fails.
	//  - [ErrInvalidCredentials] if tool execution requires auth and token is
	//    invalid.
	ExecuteTool(ctx context.Context, tool entities.Tool, args map[string]json.RawMessage, toolCallID string) (messages.MessageTool, error)
}

// ToolClientFactory creates [ToolClient] instances.
type ToolClientFactory interface {
	ToolClient() ToolClient
}

func NewToolClient(f ToolClientFactory) ToolClient {
	return f.ToolClient()
}

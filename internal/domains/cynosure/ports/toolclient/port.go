package toolclient

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
)

// ToolIDBuilder is a function that creates a tool ID for newly creating tools.
type ToolIDBuilder = func(account ids.AccountID, name string) (ids.ToolID, error)

// Port executes MCP (Model Context Protocol) operations: tool discovery and
// tool execution. Abstracts MCP server connections, protocol handling, and
// account-based access control.
type Port interface {
	// DiscoverTools retrieves available tools from an MCP server. Implements
	// the MCP tool discovery phase, returning tools with their schemas.
	//
	// Options:
	//
	//  - [WithToolIDBuilder] — sets the tool ID builder for newly creating tools.
	//
	// See next test suites to find how it works:
	//
	//  - [TestDiscoverTools] — discovering tools from MCP servers
	//
	// Throws:
	//
	//  - [ErrServerUnreachable] if server connection fails.
	//  - [ErrProtocolNotSupported] if server does not support MCP protocol.
	//  - [ErrInvalidCredentials] if OAuth token is invalid or expired.
	//  - [RequiresAuthError] if server requires auth first, and there is no
	//    data about mcp protocol yet.
	DiscoverTools(
		ctx context.Context,
		u *url.URL,
		account ids.AccountID,
		accountSlug, accountDesc string,
		opts ...DiscoverToolsOption,
	) ([]tools.RawTool, error)

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
	//  - [RequiresAuthError] if server requires auth first, and there is no
	//    data about mcp protocol yet.
	ExecuteTool(
		ctx context.Context,
		tool entities.ToolReadOnly,
		args map[string]json.RawMessage,
		toolCallID string,
	) (messages.MessageTool, error)
}

func defaultDiscoverToolsParams() discoverToolsParams {
	return discoverToolsParams{
		toolIDBuilder: func(account ids.AccountID, name string) (ids.ToolID, error) {
			return ids.RandomToolID(account)
		},
		token: nil,
	}
}

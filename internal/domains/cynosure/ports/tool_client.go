package ports

import (
	"context"
	"encoding/json"
	"net/url"

	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
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
	DiscoverTools(ctx context.Context, u *url.URL, token *oauth2.Token, account ids.AccountID, accountDesc string, opts ...DiscoverToolsOption) ([]tools.RawToolInfo, error)

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
	//  - [RequiresAuthError] if server requires auth first, and there is no data about mcp protocol yet.
	ExecuteTool(ctx context.Context, tool entities.ToolReadOnly, args map[string]json.RawMessage, toolCallID string) (messages.MessageTool, error)
}

func defaultDiscoverToolsParams() *discoverToolsParams {
	return &discoverToolsParams{
		toolIDBuilder: func(account ids.AccountID, name string) (ids.ToolID, error) {
			return ids.RandomToolID(account, ids.WithSlug(name))
		},
	}
}

// ToolClientFactory creates [ToolClient] instances.
type ToolClientFactory interface {
	ToolClient() ToolClientWrapped
}

func NewToolClient(f ToolClientFactory) ToolClientWrapped {
	return f.ToolClient()
}

type ToolClientWrapped interface {
	ToolClient

	_ToolClient()
}

type toolClientWrapped struct {
	w ToolClient

	trace trace.Tracer
}

func (t *toolClientWrapped) _ToolClient() {}


func WrapToolClient(client ToolClient, opts ...WrapToolClientOption) ToolClientWrapped {
	t := toolClientWrapped{
		w:     client,
		trace: noop.NewTracerProvider().Tracer(""),
	}
	for _, opt := range opts {
		opt.applyWrapToolClient(&t)
	}

	return &t
}

func (t *toolClientWrapped) DiscoverTools(ctx context.Context, u *url.URL, token *oauth2.Token, account ids.AccountID, accountDesc string, opts ...DiscoverToolsOption) ([]tools.RawToolInfo, error) {
	ctx, span := t.trace.Start(ctx, "DiscoverTools")
	defer span.End()

	res, err := t.w.DiscoverTools(ctx, u, token, account, accountDesc, opts...)
	if err != nil {
		span.RecordError(err)
	}

	return res, err
}

func (t *toolClientWrapped) ExecuteTool(ctx context.Context, tool entities.ToolReadOnly, args map[string]json.RawMessage, toolCallID string) (messages.MessageTool, error) {
	ctx, span := t.trace.Start(ctx, "ExecuteTool")
	defer span.End()

	res, err := t.w.ExecuteTool(ctx, tool, args, toolCallID)
	if err != nil {
		span.RecordError(err)
	}

	return res, err
}

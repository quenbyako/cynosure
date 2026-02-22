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

	// Probe checks if the specified URL points to a valid MCP server.
	// It attempts a raw connection and handshake without performing any data
	// operations.
	//
	// Throws:
	//
	//  - [ErrServerUnreachable] if server connection fails.
	//  - [ErrProtocolNotSupported] if handshake fails or protocol mismatch.
	//  - [ErrAuthRequired] if server acknowledges MCP but requires auth.
	Probe(ctx context.Context, u *url.URL) error
}

// ToolClientFactory creates [ToolClient] instances.
type ToolClientFactory interface {
	ToolClient() ToolClientWrapped
}

func NewToolClient(f ToolClientFactory) ToolClient {
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

type WrapToolClientOption func(*toolClientWrapped)

func WithToolClientTrace(trace trace.Tracer) WrapToolClientOption {
	return func(p *toolClientWrapped) { p.trace = trace }
}

func WrapToolClient(client ToolClient, opts ...WrapToolClientOption) ToolClientWrapped {
	t := toolClientWrapped{
		w:     client,
		trace: noop.NewTracerProvider().Tracer(""),
	}
	for _, opt := range opts {
		opt(&t)
	}

	return &t
}

func (t *toolClientWrapped) DiscoverTools(ctx context.Context, u *url.URL, token *oauth2.Token, account ids.AccountID, accountDesc string) ([]tools.RawToolInfo, error) {
	ctx, span := t.trace.Start(ctx, "DiscoverTools")
	defer span.End()

	res, err := t.w.DiscoverTools(ctx, u, token, account, accountDesc)
	if err != nil {
		span.RecordError(err)
	}

	return res, err
}

func (t *toolClientWrapped) ExecuteTool(ctx context.Context, tool entities.Tool, args map[string]json.RawMessage, toolCallID string) (messages.MessageTool, error) {
	ctx, span := t.trace.Start(ctx, "ExecuteTool")
	defer span.End()

	res, err := t.w.ExecuteTool(ctx, tool, args, toolCallID)
	if err != nil {
		span.RecordError(err)
	}

	return res, err
}

func (t *toolClientWrapped) Probe(ctx context.Context, u *url.URL) error {
	ctx, span := t.trace.Start(ctx, "Probe")
	defer span.End()

	err := t.w.Probe(ctx, u)
	if err != nil {
		span.RecordError(err)
	}

	return err
}

package mcp

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/accounts"
)

type Controller struct {
	accounts *accounts.Usecase
}

type newParams struct {
	logger         slog.Handler
	allowedIssuers []string
}

type NewOption func(*newParams)

func WithLogger(logger slog.Handler) NewOption {
	return func(p *newParams) { p.logger = logger }
}

func WithAllowedIssuers(issuers ...string) NewOption {
	return func(p *newParams) { p.allowedIssuers = issuers }
}

func New(accounts *accounts.Usecase, impl mcp.Implementation, opts ...NewOption) http.Handler {
	p := newParams{
		logger: slog.DiscardHandler,
	}
	for _, opt := range opts {
		opt(&p)
	}

	ctrl := &Controller{
		accounts: accounts,
	}
	if err := ctrl.validate(); err != nil {
		panic(err)
	}

	srv := mcp.NewServer(&impl, &mcp.ServerOptions{
		Logger: slog.New(p.logger),
	})

	route(srv, ctrl)

	mux := http.NewServeMux()
	mux.Handle("/mcp", mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server { return srv }, &mcp.StreamableHTTPOptions{
		JSONResponse: true,
	}))

	return Middleware(p.allowedIssuers, slog.New(p.logger))(mux)
}

func (c *Controller) validate() error {
	if c.accounts == nil {
		return errors.New("accounts is nil")
	}
	return nil
}

func route(srv *mcp.Server, c *Controller) {
	// --- MCP Servers & Accounts Management ---
	register(srv, "authorize_mcp_server", "", "Registers an MCP server by URL. Returns either an auth link or a direct account ID.", c.AuthorizeMcpServer)
	register(srv, "search_mcp_servers", "", "Searches for registered public MCP servers using a text query.", c.SearchMcpServers)
	register(srv, "list_mcp_accounts", "", "Returns a list of registered and active MCP accounts for the current user.", c.ListMcpAccounts)
	register(srv, "disable_mcp_account", "", "Deactivates an MCP account, preventing tools from being used.", c.DisableMcpAccount)
	register(srv, "reactivate_mcp_account", "", "Reactivates a previously disabled MCP account.", c.ReactivateMcpAccount)

	// --- Tools Discovery ---
	register(srv, "list_mcp_tools", "", "Lists all available tools from all active MCP accounts.", c.ListMcpTools)
	register(srv, "search_mcp_tools", "", "Search for tools across all active MCP accounts by query.", c.SearchMcpTools)

	// --- Agents Management ---
	register(srv, "create_agent", "", "Creates a new autonomous agent with specified prompt and model.", c.CreateAgent)
	register(srv, "update_agent", "", "Updates parameters of an existing autonomous agent.", c.UpdateAgent)
	register(srv, "list_agents", "", "List all agents belonging to the current user.", c.ListAgents)
	register(srv, "disable_agent", "", "Deactivates an agent.", c.DisableAgent)

}

const requestTimeout = 30 * time.Second

// regcister is a clean wrapper around mcp.AddTool that hides the JSON-RPC boilerplate.
func register[In, Out any](srv *mcp.Server, name, title, desc string, h func(context.Context, In) (Out, error)) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        name,
		Description: desc,
		Title:       title,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error) {
		ctx, cancel := context.WithTimeout(ctx, requestTimeout)
		defer cancel()

		out, err := h(ctx, in)
		return nil, out, err
	})
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

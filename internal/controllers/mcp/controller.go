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

func buildNewParams(opts ...NewOption) newParams {
	params := newParams{
		logger:         slog.DiscardHandler,
		allowedIssuers: nil,
	}
	for _, opt := range opts {
		opt(&params)
	}

	return params
}

func (p *newParams) buildServerOptions() *mcp.ServerOptions {
	return &mcp.ServerOptions{
		Instructions: "",
		//nolint:forbidigo // there is a reason to use it
		Logger:                      slog.New(p.logger),
		InitializedHandler:          nil,
		PageSize:                    0,
		RootsListChangedHandler:     nil,
		ProgressNotificationHandler: nil,
		CompletionHandler:           nil,
		KeepAlive:                   0,
		SubscribeHandler:            nil,
		UnsubscribeHandler:          nil,
		Capabilities:                nil,
		HasPrompts:                  false,
		HasResources:                false,
		HasTools:                    false,
		SchemaCache:                 nil,
		GetSessionID:                nil,
	}
}

type NewOption func(*newParams)

func WithLogger(logger slog.Handler) NewOption {
	return func(p *newParams) { p.logger = logger }
}

func WithAllowedIssuers(issuers ...string) NewOption {
	return func(p *newParams) { p.allowedIssuers = issuers }
}

func New(
	accountsUsecase *accounts.Usecase,
	impl mcp.Implementation,
	opts ...NewOption,
) (
	http.Handler,
	error,
) {
	params := buildNewParams(opts...)

	ctrl := &Controller{
		accounts: accountsUsecase,
	}

	if err := ctrl.validate(); err != nil {
		return nil, err
	}

	srv := mcp.NewServer(&impl, params.buildServerOptions())

	route(srv, ctrl)

	mux := buildMux(srv)

	return Middleware(params.allowedIssuers, params.logger)(mux), nil
}

func buildMux(srv *mcp.Server) *http.ServeMux {
	mux := http.NewServeMux()

	mux.Handle("/mcp", mcp.NewStreamableHTTPHandler(
		func(r *http.Request) *mcp.Server { return srv },
		&mcp.StreamableHTTPOptions{
			JSONResponse:   true,
			Stateless:      false,
			Logger:         nil,
			EventStore:     nil,
			SessionTimeout: 0,
		},
	))

	return mux
}

//nolint:err113 // validation will always throw specific unhandled errors.
func (c *Controller) validate() error {
	if c.accounts == nil {
		return errors.New("accounts usecase is nil")
	}

	return nil
}

func route(srv *mcp.Server, ctrl *Controller) {
	// --- MCP Servers & Accounts Management ---
	register(srv, authorizeMcpServerName, "", authorizeMcpServerDesc, ctrl.AuthorizeMcpServer)
	register(srv, searchMcpServersName, "", searchMcpServersDesc, ctrl.SearchMcpServers)
	register(srv, listMcpAccountsName, "", listMcpAccountsDesc, ctrl.ListMcpAccounts)
	register(srv, disableMcpAccountName, "", disableMcpAccountDesc, ctrl.DisableMcpAccount)
	register(srv, reactivateMcpAccountName, "", reactivateMcpAccountDesc, ctrl.ReactivateMcpAccount)

	// --- Tools Discovery ---
	register(srv, listMcpToolsName, "", listMcpToolsDesc, ctrl.ListMcpTools)
	register(srv, searchMcpToolsName, "", searchMcpToolsDesc, ctrl.SearchMcpTools)

	// --- Agents Management ---
	register(srv, createAgentName, "", createAgentDesc, ctrl.CreateAgent)
	register(srv, updateAgentName, "", updateAgentDesc, ctrl.UpdateAgent)
	register(srv, listAgentsName, "", listAgentsDesc, ctrl.ListAgents)
	register(srv, disableAgentName, "", disableAgentDesc, ctrl.DisableAgent)
}

const (
	requestTimeout = 30 * time.Second
)

type handlerFunc[In, Out any] = func(context.Context, In) (Out, error)

// regcister is a clean wrapper around mcp.AddTool that hides the JSON-RPC boilerplate.
func register[In, Out any](srv *mcp.Server, name, title, desc string, handle handlerFunc[In, Out]) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:         name,
		Description:  desc,
		Title:        title,
		Meta:         nil,
		Annotations:  nil,
		InputSchema:  nil,
		OutputSchema: nil,
		Icons:        nil,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error) {
		ctx, cancel := context.WithTimeout(ctx, requestTimeout)
		defer cancel()

		out, err := handle(ctx, in)

		return nil, out, err
	})
}

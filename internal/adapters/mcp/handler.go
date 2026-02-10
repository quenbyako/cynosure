package mcp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	cache "github.com/quenbyako/cynosure/contrib/sf-cache"
	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
)

type (
	AccountTokenFunc func(context.Context, ids.AccountID) (entities.ServerConfigReadOnly, *oauth2.Token, error)
	ServerInfoFunc   func(context.Context, ids.ServerID) (entities.ServerConfigReadOnly, error)
	RefreshTokenFunc func(context.Context, *oauth2.Token) (*oauth2.Token, error)
	SaveTokenFunc    func(context.Context, ids.AccountID, *oauth2.Token) error
)

var clientImpl = &mcp.Implementation{
	Name:    "cynosure",
	Version: "0.1.0",
}

type Handler struct {
	clients *cache.Cache[ids.AccountID, *asyncClient]
}

var _ ports.ToolClientFactory = (*Handler)(nil)

func (h *Handler) ToolClient() ports.ToolClient { return h }

func NewHandler(
	refresher RefreshTokenFunc,
	storage SaveTokenFunc,
	accountToken AccountTokenFunc,
) *Handler {
	return &Handler{
		clients: cache.New(
			cacheConstructor(refresher, storage, accountToken, 10*time.Second),
			cacheDestructor(),
			5,
			10*time.Minute,
		),
	}
}

type asyncClient struct {
	session            *mcp.ClientSession
	cancel             context.CancelFunc
	usedProtocol       tools.Protocol   // Which protocol was successfully used
	attemptedProtocols []tools.Protocol // All protocols attempted
}

// newAsyncClient creates an MCP client with protocol fallback (Streamable → SSE).
// It tries StreamableClientTransport first, then falls back to SSEClientTransport
// only on protocol errors. Infrastructure and auth errors fail immediately.
// Returns the client with information about which protocol succeeded.
func newAsyncClient(ctx context.Context, u *url.URL, httpClient *http.Client, preferredProtocol tools.Protocol) (*asyncClient, error) {
	clientCtx, clientCancel := context.WithCancel(context.WithoutCancel(ctx))

	var attemptedProtocols []tools.Protocol
	var usedProtocol tools.Protocol
	var session *mcp.ClientSession
	var err error

	// Helper to try HTTP
	tryHTTP := func() error {
		attemptedProtocols = append(attemptedProtocols, tools.ProtocolHTTP)
		s, e := connectWithTransport(clientCtx, &mcp.StreamableClientTransport{
			Endpoint:   u.String(),
			HTTPClient: httpClient,
			MaxRetries: 0, // No retries - fail fast for protocol detection
		})
		if e == nil {
			session = s
			usedProtocol = tools.ProtocolHTTP
		}
		return e
	}

	// Helper to try SSE
	trySSE := func() error {
		attemptedProtocols = append(attemptedProtocols, tools.ProtocolSSE)
		s, e := connectWithTransport(clientCtx, &mcp.SSEClientTransport{
			Endpoint:   u.String(),
			HTTPClient: httpClient,
		})
		if e == nil {
			session = s
			usedProtocol = tools.ProtocolSSE
		}
		return e
	}

	// Logic based on preferred protocol
	switch preferredProtocol {
	case tools.ProtocolHTTP:
		// Trust preference: HTTP only
		err = tryHTTP()

	case tools.ProtocolSSE:
		// Trust preference: SSE only
		err = trySSE()

	default: // Unknown or Invalid
		// Fallback logic: Try HTTP first
		err = tryHTTP()
		if err != nil {
			// On protocol error, fallback to SSE
			classifiedErr := ClassifyTransportError(err)
			if e := new(ProtocolError); errors.As(classifiedErr, &e) {
				err = trySSE()
			}
		}
	}

	// If failed, cleanup and return
	if err != nil {
		clientCancel()
		return nil, fmt.Errorf("connecting to %v: %w", u.String(), err)
	}

	return &asyncClient{
		session:            session,
		cancel:             clientCancel,
		usedProtocol:       usedProtocol,
		attemptedProtocols: attemptedProtocols,
	}, nil
}

// connectWithTransport attempts to connect using the specified transport.
// Returns the session on success, or an error that can be classified for fallback.
func connectWithTransport(ctx context.Context, transport mcp.Transport) (*mcp.ClientSession, error) {
	client := mcp.NewClient(clientImpl, &mcp.ClientOptions{
		KeepAlive: 10 * time.Second,
	})

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, err
	}

	return session, nil
}

func (c *asyncClient) Close() error {
	err := c.session.Close()
	// even if we received error, we should cancel context anyway.
	c.cancel()

	return err
}

func cacheConstructor(
	refresher RefreshTokenFunc,
	storage SaveTokenFunc,
	accountToken AccountTokenFunc,
	refreshTimeout time.Duration,
) cache.ConstructorFunc[ids.AccountID, *asyncClient] {
	return func(ctx context.Context, account ids.AccountID) (*asyncClient, error) {
		server, session, err := accountToken(ctx, account)
		if err != nil {
			return nil, fmt.Errorf("retrieving session for account %v: %w", account.ID().String(), err)
		}

		httpClient := http.DefaultClient
		if session != nil {
			// WithoutCancel preserves deadline but detaches from request cancellation.
			// This ensures token refresh completes even if the user cancels the request,
			// but still respects timeout constraints.
			httpClient = oauth2.NewClient(ctx, NewRefresher(
				context.WithoutCancel(ctx),
				session,
				refresher,
				storage,
				account,
				server,
				refreshTimeout,
			))
		}

		return newAsyncClient(ctx, server.SSELink(), httpClient, server.PreferredProtocol())
	}
}

func cacheDestructor() cache.DestructorFunc[ids.AccountID, *asyncClient] {
	return func(_ ids.AccountID, c *asyncClient) {
		if err := c.Close(); err != nil {
			panic(fmt.Errorf("closing client: %w (i'll handle error later)", err))
		}
	}
}

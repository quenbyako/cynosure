package primitive

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	cache "github.com/quenbyako/cynosure/contrib/sf-cache"

	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/ids"
)

type Handler struct {
	clients *cache.Cache[ids.AccountID, *asyncClient]

	servers  ports.ServerStorage
	accounts ports.AccountStorage
}

var _ ports.ToolManagerFactory = (*Handler)(nil)

func (h *Handler) ToolManager() ports.ToolManager { return h }

func NewHandler(auth ports.OAuthHandler, servers ports.ServerStorage, accounts ports.AccountStorage) *Handler {
	constructor := func(ctx context.Context, account ids.AccountID) (*asyncClient, error) {
		session, err := accounts.GetAccount(ctx, account)
		if err != nil {
			return nil, fmt.Errorf("retrieving session for account %v: %w", account.ID().String(), err)
		}

		serverInfo, err := servers.GetServerInfo(ctx, account.Server())
		if err != nil {
			return nil, fmt.Errorf("retrieving server info for account %v: %w", account.ID().String(), err)
		}

		httpClient := http.DefaultClient
		if session.Token() != nil {
			httpClient = oauth2.NewClient(ctx, newRefresher(
				context.TODO(),
				auth,
				accounts,
				session,
				serverInfo,
			))
		}

		return newAsyncClient(ctx, serverInfo.SSELink, httpClient)
	}

	destructor := func(_ ids.AccountID, c *asyncClient) {
		if err := c.Close(); err != nil {
			panic(fmt.Errorf("closing client: %w (i'll handle error later)", err))
		}
	}

	return &Handler{
		clients: cache.New(
			constructor,
			destructor,
			5,
			10*time.Minute,
		),
		servers:  servers,
		accounts: accounts,
	}
}

type asyncClient struct {
	session *mcp.ClientSession
	cancel  context.CancelFunc
}

var clientImpl = &mcp.Implementation{
	Name:    "cynosure",
	Version: "0.1.0",
}

func newAsyncClient(ctx context.Context, u *url.URL, httpCLient *http.Client) (*asyncClient, error) {
	clientCtx, clientCancel := context.WithCancel(context.WithoutCancel(ctx))

	client := mcp.NewClient(clientImpl, &mcp.ClientOptions{
		KeepAlive: 10 * time.Second,
	})

	session, err := client.Connect(clientCtx, &mcp.SSEClientTransport{
		Endpoint:   u.String(),
		HTTPClient: httpCLient,
	}, nil)
	if err != nil {
		clientCancel()
		return nil, fmt.Errorf("connecting to %v: %w", u.String(), err)
	}

	return &asyncClient{
		session: session,
		cancel:  clientCancel,
	}, nil
}

func (c *asyncClient) Close() error {
	err := c.session.Close()
	// even if we received error, we should cancel context anyway.
	c.cancel()

	return err
}

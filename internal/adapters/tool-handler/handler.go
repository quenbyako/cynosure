package primitive

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	cache "tg-helper/contrib/sf-cache"

	"github.com/k0kubun/pp/v3"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport/sse"
	"github.com/mark3labs/mcp-go/mcp"
	"golang.org/x/oauth2"

	"tg-helper/internal/domains/components/ids"
	"tg-helper/internal/domains/ports"
)

type Handler struct {
	clients *cache.Cache[ids.AccountID, *asyncClient]

	servers  ports.ServerStorage
	accounts ports.AccountStorage
}

var _ ports.ToolManager = (*Handler)(nil)

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

		return newAsyncClient(ctx, serverInfo.SSELink, sse.WithHTTPClient(httpClient))
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
	c      *client.Client
	cancel context.CancelFunc
	wg     *sync.WaitGroup
}

func newAsyncClient(ctx context.Context, u *url.URL, opts ...sse.ConnectOpt) (*asyncClient, error) {
	clientCtx, clientCancel := context.WithCancel(context.WithoutCancel(ctx))
	context.AfterFunc(ctx, func() { fmt.Println("PRIMARY CONTEXT IS DONE") })
	context.AfterFunc(clientCtx, func() { fmt.Println("CLIENT CONTEXT COMPLETED!") })

	transport, err := sse.Connect(clientCtx, u, opts...)
	if err != nil {
		clientCancel()
		return nil, err
	}
	c := client.NewClient(transport)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := c.Run(clientCtx); err != nil && !errors.Is(err, context.Canceled) {
			panic(err)
		}
		pp.Println("i'm finished!")
	}()

	capabilities, err := c.Initialize(ctx, mcp.InitializeParams{
		ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
		ClientInfo:      mcp.Implementation{Name: "tg-helper"},
	})
	if err != nil {
		clientCancel()
		return nil, err
	}

	fmt.Println("Initialized client with capabilities:", capabilities)

	return &asyncClient{
		c:      c,
		cancel: clientCancel,
		wg:     &wg,
	}, nil
}

func (ac *asyncClient) Close() error {
	pp.Println("closing client!")

	ac.cancel()
	ac.wg.Wait()
	return nil
}

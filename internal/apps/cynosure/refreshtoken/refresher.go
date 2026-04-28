// Package refreshtoken provides OAuth token refresh lifecycle management.
package refreshtoken

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/alitto/pond/v2"
	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/oauthhandler"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

type RefreshConstructor struct {
	oauthPort       oauthhandler.Port
	accounts        ports.AccountStorage
	servers         ports.ServerStorage
	pool            pond.ResultPool[*oauth2.Token]
	maxTaskDuration time.Duration
	workersCount    int
	poolMu          sync.RWMutex
}

const (
	defaultMaxTaskDuration = 30 * time.Second
)

func NewConstructor(
	oauthPort oauthhandler.Port,
	accounts ports.AccountStorage,
	servers ports.ServerStorage,
	workersCount int,
) *RefreshConstructor {
	return &RefreshConstructor{
		oauthPort:       oauthPort,
		accounts:        accounts,
		servers:         servers,
		maxTaskDuration: defaultMaxTaskDuration,
		pool:            nil,
		workersCount:    workersCount,
		poolMu:          sync.RWMutex{},
	}
}

func (c *RefreshConstructor) Run(ctx context.Context) error {
	c.poolMu.Lock()
	if c.pool != nil {
		//nolint:forbidigo // system-wide failure, absolutely unsafe to ignore
		panic("already running")
	}

	// using WithoutCancel to prevent pool from being stopped when the main
	// context is canceled. main context works below. Here we are just saving
	// values that might be necessary for tasks.
	c.pool = pond.NewResultPool[*oauth2.Token](c.workersCount,
		pond.WithContext(context.WithoutCancel(ctx)),
	)
	c.poolMu.Unlock()

	// Wait for shutdown signal
	<-ctx.Done()
	c.pool.StopAndWait()

	return nil
}

func (c *RefreshConstructor) Token(ctx context.Context, accountID ids.AccountID) (
	entities.ServerConfigReadOnly, *oauth2.Token, error,
) {
	account, err := c.accounts.GetAccount(ctx, accountID)
	if err != nil {
		return nil, nil, fmt.Errorf("getting account: %w", err)
	}

	server, err := c.servers.GetServerInfo(ctx, accountID.Server())
	if err != nil {
		return nil, nil, fmt.Errorf("getting server info: %w", err)
	}

	return server, account.Token(), nil
}

func (c *RefreshConstructor) Build(
	id ids.AccountID,
	config *oauth2.Config,
	token *oauth2.Token,
	internal bool,
) (oauth2.TokenSource, error) {
	if token.RefreshToken == "" {
		return nil, ErrRefreshTokenNotSet
	}

	return &tokenSource{
		base: c,

		id:       id,
		config:   config,
		token:    token,
		internal: internal,
	}, nil
}

func (c *RefreshConstructor) GetToken(
	id ids.AccountID,
	config *oauth2.Config,
	token *oauth2.Token,
	internal bool,
) (*oauth2.Token, error) {
	c.poolMu.RLock()
	pool := c.pool
	c.poolMu.RUnlock()

	if pool == nil {
		return nil, pond.ErrPoolStopped
	}

	result := pool.SubmitErr(func() (*oauth2.Token, error) {
		return c.getToken(id, config, token, internal)
	})

	newToken, err := result.Wait()
	if err != nil {
		return nil, fmt.Errorf("waiting for token refresh: %w", err)
	}

	return newToken, nil
}

// TODO: create error like "probably you need to sign in again" in case when
// token refreshed, but didn't save it to db?
func (c *RefreshConstructor) getToken(
	id ids.AccountID, config *oauth2.Config, token *oauth2.Token, internal bool,
) (*oauth2.Token, error) {
	ctx, cancel := context.WithTimeout(c.pool.Context(), c.maxTaskDuration)
	defer cancel()

	var opts []oauthhandler.RefreshTokenOption
	if internal {
		opts = append(opts, oauthhandler.WithInternalConnection())
	}

	newToken, err := c.oauthPort.RefreshToken(ctx, config, token, opts...)
	if err != nil {
		return nil, fmt.Errorf("refreshing token: %w", err)
	}

	if err := c.updateAccountToken(ctx, id, newToken); err != nil {
		return nil, err
	}

	return newToken, nil
}

func (c *RefreshConstructor) updateAccountToken(
	ctx context.Context, id ids.AccountID, token *oauth2.Token,
) error {
	account, err := c.accounts.GetAccount(ctx, id)
	if err != nil {
		return fmt.Errorf("getting account: %w", err)
	}

	if err := account.UpdateToken(token); err != nil {
		return fmt.Errorf("updating token: %w", err)
	}

	if err := c.accounts.SaveAccount(ctx, account); err != nil {
		return fmt.Errorf("saving account: %w", err)
	}

	return nil
}

type tokenSource struct {
	base *RefreshConstructor

	config   *oauth2.Config
	token    *oauth2.Token
	id       ids.AccountID
	internal bool
}

var _ oauth2.TokenSource = (*tokenSource)(nil)

func (t *tokenSource) Token() (*oauth2.Token, error) {
	return t.base.GetToken(t.id, t.config, t.token, t.internal)
}

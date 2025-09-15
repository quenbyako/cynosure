package primitive

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"golang.org/x/oauth2"

	"tg-helper/internal/domains/entities"
	"tg-helper/internal/domains/ports"
)

type tokenRefresher struct {
	ctx context.Context // used to get HTTP requests
	m   sync.RWMutex

	auth    ports.OAuthHandler
	storage ports.AccountStorage

	account *entities.Account
	server  *ports.ServerInfo
}

var _ oauth2.TokenSource = (*tokenRefresher)(nil)

func newRefresher(
	ctx context.Context,
	auth ports.OAuthHandler,
	storage ports.AccountStorage,
	account *entities.Account,
	server *ports.ServerInfo,
) *tokenRefresher {
	return &tokenRefresher{
		ctx:     ctx,
		auth:    auth,
		storage: storage,
		account: account,
		server:  server,
	}
}

func (t *tokenRefresher) Token() (*oauth2.Token, error) {
	if t.account.Token().RefreshToken == "" {
		return nil, errors.New("token expired and refresh token is not set")
	}

	token, err := t.auth.RefreshToken(t.ctx, t.server.AuthConfig, t.account.Token())
	if err != nil {
		return nil, fmt.Errorf("refreshing token: %w", err)
	}

	if err := t.account.UpdateToken(token); err != nil {
		return nil, fmt.Errorf("updating account token: %w", err)
	}

	if err := t.storage.SaveAccount(t.ctx, t.account); err != nil {
		return nil, fmt.Errorf("updating account in storage: %w", err)
	}

	return token, nil
}

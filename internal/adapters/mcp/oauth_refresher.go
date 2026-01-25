package mcp

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

type tokenRefresher struct {
	ctx context.Context
	m   sync.RWMutex

	refreshToken RefreshTokenFunc
	saveToken    SaveTokenFunc

	account      ids.AccountID
	token        *oauth2.Token
	tokenTimeout time.Duration

	server entities.ServerConfigReadOnly
}

var _ oauth2.TokenSource = (*tokenRefresher)(nil)

func NewRefresher(
	ctx context.Context,
	token *oauth2.Token,
	auth RefreshTokenFunc,
	storage SaveTokenFunc,
	account ids.AccountID,
	server entities.ServerConfigReadOnly,
	tokenTimeout time.Duration,
) *tokenRefresher {
	return &tokenRefresher{
		m: sync.RWMutex{},

		ctx:          ctx,
		refreshToken: auth,
		saveToken:    storage,
		account:      account,
		server:       server,
		tokenTimeout: tokenTimeout,
		token:        token,
	}
}

func (t *tokenRefresher) Token() (*oauth2.Token, error) {
	if t.token.RefreshToken == "" {
		return nil, errors.New("token expired and refresh token is not set")
	}

	ctx, cancel := context.WithTimeout(t.ctx, t.tokenTimeout)
	defer cancel()

	token, err := t.refreshToken(ctx, t.token)
	if err != nil {
		return nil, fmt.Errorf("refreshing token: %w", err)
	}

	if err := t.saveToken(ctx, t.account, token); err != nil {
		return nil, fmt.Errorf("updating account in storage: %w", err)
	}

	return token, nil
}

package mcp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

type tokenRefresher struct {
	ctx           context.Context
	saveToken     SaveTokenFunc
	account       ids.AccountID
	tokenTimeout  time.Duration
	config        *oauth2.Config
	originalToken *oauth2.Token
}

var _ oauth2.TokenSource = (*tokenRefresher)(nil)

func NewRefresher(
	ctx context.Context,
	token *oauth2.Token,
	storage SaveTokenFunc,
	account ids.AccountID,
	config *oauth2.Config,
	tokenTimeout time.Duration,
) oauth2.TokenSource {
	return &tokenRefresher{
		ctx:           ctx,
		saveToken:     storage,
		account:       account,
		config:        config,
		tokenTimeout:  tokenTimeout,
		originalToken: token,
	}
}

func (t *tokenRefresher) Token() (*oauth2.Token, error) {
	// If original token doesn't have a refresh token, we can't refresh.
	if t.originalToken.RefreshToken == "" {
		return nil, errors.New("token expired and refresh token is not set")
	}

	ctx, cancel := context.WithTimeout(t.ctx, t.tokenTimeout)
	defer cancel()

	newToken, err := t.config.TokenSource(ctx, t.originalToken).Token()
	if err != nil {
		return nil, fmt.Errorf("refreshing token: %w", err)
	}

	if err := t.saveToken(ctx, t.account, newToken); err != nil {
		return nil, fmt.Errorf("updating account in storage: %w", err)
	}

	return newToken, nil
}

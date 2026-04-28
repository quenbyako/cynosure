package mcp

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

// RefreshTokenFunc is a function that refreshes an OAuth 2.0 token.
type RefreshTokenFunc func(context.Context, *oauth2.Config, *oauth2.Token) (*oauth2.Token, error)

type tokenRefresher struct {
	//nolint:containedctx // ctx is required for context cancellation
	ctx           context.Context
	saveToken     SaveTokenFunc
	config        *oauth2.Config
	originalToken *oauth2.Token
	refresh       RefreshTokenFunc
	tokenTimeout  time.Duration
	account       ids.AccountID
}

var _ oauth2.TokenSource = (*tokenRefresher)(nil)

// NewRefresher creates a new oauth2.TokenSource that refreshes tokens.
func NewRefresher(
	ctx context.Context,
	token *oauth2.Token,
	storage SaveTokenFunc,
	account ids.AccountID,
	config *oauth2.Config,
	tokenTimeout time.Duration,
	refresh RefreshTokenFunc,
) oauth2.TokenSource {
	return &tokenRefresher{
		ctx:           ctx,
		saveToken:     storage,
		account:       account,
		config:        config,
		tokenTimeout:  tokenTimeout,
		originalToken: token,
		refresh:       refresh,
	}
}

func (t *tokenRefresher) Token() (*oauth2.Token, error) {
	if t.originalToken.RefreshToken == "" {
		return nil, ErrRefreshTokenNotSet
	}

	ctx, cancel := context.WithTimeout(t.ctx, t.tokenTimeout)
	defer cancel()

	newToken, err := t.refresh(ctx, t.config, t.originalToken)
	if err != nil {
		return nil, fmt.Errorf("refreshing token: %w", err)
	}

	if err := t.saveToken(ctx, t.account, newToken); err != nil {
		return nil, fmt.Errorf("updating account in storage: %w", err)
	}

	return newToken, nil
}

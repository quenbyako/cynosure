package ports

import (
	"context"
	"net/url"
	"time"

	"golang.org/x/oauth2"
)

type OAuthHandler interface {
	// RegisterClient регистрирует динамический клиент в случае, если сервер
	// поддерживает такой способ.
	RegisterClient(ctx context.Context, u *url.URL, clientName string, redirect *url.URL) (cfg *oauth2.Config, expiresAt time.Time, err error)
	// RefreshToken обновляет токен доступа
	RefreshToken(ctx context.Context, config *oauth2.Config, token *oauth2.Token) (*oauth2.Token, error)
	// Exchange обменивает код авторизации на токен доступа
	Exchange(ctx context.Context, config *oauth2.Config, code string, verifier []byte) (*oauth2.Token, error)
}

type OAuthHandlerFactory interface {
	OAuthHandler() OAuthHandler
}

func NewOAuthHandler(factory OAuthHandlerFactory) OAuthHandler { return factory.OAuthHandler() }

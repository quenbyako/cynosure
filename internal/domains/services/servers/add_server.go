package servers

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"tg-helper/internal/domains/components/ids"
	"tg-helper/internal/domains/ports"
)

func (s *Service) AddServer(ctx context.Context, server ids.ServerID, u *url.URL) error {
	ctx, span := s.trace.Start(ctx, "Service.AddServer")
	defer span.End()

	_, err := s.servers.GetServerInfo(ctx, server) // just to see if it already exists
	if err == nil {

		return errors.New("server already registered")
	} else if !errors.Is(err, ports.ErrNotFound) {
		return fmt.Errorf("failed to lookup server: %w", err)
	}

	cfg, expires, err := s.oauth.RegisterClient(ctx, u, s.oauthClientName, s.authRedirectURL)
	if errors.Is(err, ports.ErrAuthUnsupported) {
		// auth is not required, just registering it.
		//
		// TODO: если реально нет авторизации, то mcp сервер по идее должен
		// отвечать с пустым токеном, надо проверять этот момент в порте
		// коннектов

	} else if err != nil {
		return fmt.Errorf("failed to register OAuth client: %w", err)
	}

	return s.servers.AddServer(ctx, server, ports.ServerInfo{
		SSELink:          u,
		AuthConfig:       cfg,
		ConfigExpiration: expires,
	})
}

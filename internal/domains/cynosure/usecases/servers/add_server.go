package servers

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/ids"
)

func (s *Service) AddServer(ctx context.Context, id ids.ServerID, u *url.URL) error {
	ctx, span := s.trace.Start(ctx, "Service.AddServer")
	defer span.End()

	_, err := s.servers.GetServerInfo(ctx, id) // just to see if it already exists
	if err == nil {
		return errors.New("server already registered")
	} else if !errors.Is(err, ports.ErrNotFound) {
		return fmt.Errorf("failed to lookup server: %w", err)
	}

	var opts []entities.ServerConfigOption

	cfg, expires, err := s.oauth.RegisterClient(ctx, u, s.oauthClientName, s.authRedirectURL)
	if err == nil {
		opts = append(opts, entities.WithAuthConfig(cfg))
		if !expires.IsZero() {
			opts = append(opts, entities.WithExpiration(expires))
		}
	} else if errors.Is(err, ports.ErrAuthUnsupported) {
		// auth is not required, just registering it.
		//
		// TODO: если реально нет авторизации, то mcp сервер по идее должен
		// отвечать с пустым токеном, надо проверять этот момент в порте
		// коннектов
	} else {
		return fmt.Errorf("failed to register OAuth client: %w", err)
	}

	server, err := entities.NewServerConfig(id, u, opts...)
	if err != nil {
		return fmt.Errorf("creating server config: %w", err)
	}

	return s.servers.AddServer(ctx, server)
}

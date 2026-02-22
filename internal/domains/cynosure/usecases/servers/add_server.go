package servers

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

func (s *Usecase) AddServer(ctx context.Context, u *url.URL) (id ids.ServerID, authRequired bool, err error) {
	ctx, span := s.trace.Start(ctx, "Service.AddServer")
	defer span.End()

	server, err := s.servers.LookupByURL(ctx, u) // just to see if it already exists
	if err == nil {
		return server.ID(), server.AuthConfig() != nil, nil
	} else if !errors.Is(err, ports.ErrNotFound) {
		return ids.ServerID{}, false, fmt.Errorf("failed to lookup server: %w", err)
	}

	var opts []entities.ServerConfigOption

	// Step 1: Probe MCP server anonymously to check if it's reachable and if it requires auth
	mcpErr := s.tools.Probe(ctx, u)

	// If it's a protocol mismatch or auth error, we might still be able to use it if we can register
	isMcp := mcpErr == nil || errors.Is(mcpErr, ports.ErrAuthRequired) || errors.Is(mcpErr, ports.ErrProtocolNotSupported)
	if !isMcp {
		return ids.ServerID{}, false, fmt.Errorf("failed to probe MCP server: %w", mcpErr)
	}

	// Step 2: Try to register OAuth client
	cfg, expires, oauthErr := s.oauth.RegisterClient(ctx, u, s.oauthClientName, s.authRedirectURL)
	if oauthErr == nil {
		opts = append(opts, entities.WithAuthConfig(cfg))
		if !expires.IsZero() {
			opts = append(opts, entities.WithExpiration(expires))
		}
		authRequired = true
	} else if e := new(ports.RequiresAuthError); errors.As(oauthErr, &e) {
		// already has config, or something else.
		// this case should probably be handled by checking existing config first,
		// but for now we follow the existing pattern.
		// Actually, RegisterClient should probably return the config if it already exists/is supported but static.
	} else if errors.Is(oauthErr, ports.ErrAuthUnsupported) {
		// auth is not supported. If MCP said auth is required, we fail.
		if errors.Is(mcpErr, ports.ErrAuthRequired) {
			return ids.ServerID{}, false, fmt.Errorf("authentication required but server does not support registration: %w", mcpErr)
		}
		authRequired = false
	} else {
		return ids.ServerID{}, false, fmt.Errorf("failed to register OAuth client: %w", oauthErr)
	}

	id = ids.RandomServerID()

	server, err = entities.NewServerConfig(id, u, opts...)
	if err != nil {
		return ids.ServerID{}, false, fmt.Errorf("creating server config: %w", err)
	}

	if err := s.servers.SetServer(ctx, server); err != nil {
		return ids.ServerID{}, false, fmt.Errorf("failed to set server: %w", err)
	}

	return server.ID(), authRequired, nil
}

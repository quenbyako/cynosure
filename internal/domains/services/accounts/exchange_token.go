package accounts

import (
	"context"
	"fmt"

	"tg-helper/internal/domains/components/oauth"
)

func (s *Service) ExchangeToken(ctx context.Context, exchangeToken, stateStr string) error {
	ctx, span := s.trace.Start(ctx, "Service.ExchangeToken")
	defer span.End()

	if stateStr == "" {
		return fmt.Errorf("state parameter is required")
	}

	if exchangeToken == "" {
		return fmt.Errorf("exchange token is required")
	}

	state, err := oauth.StateFromToken(stateStr, s.key)
	if err != nil {
		return fmt.Errorf("invalid state: %w", err)
	}

	server, err := s.servers.GetServerInfo(ctx, state.Account().Server())
	if err != nil {
		return fmt.Errorf("getting server info: %w", err)
	}

	token, err := s.oauth.Exchange(ctx, server.AuthConfig, exchangeToken, state.Challenge())
	if err != nil {
		return fmt.Errorf("exchanging token: %w", err)
	}

	return s.tools.RegisterTools(ctx, state.Account(), state.Name(), state.Description(), token)
}

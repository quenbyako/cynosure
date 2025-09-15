package accounts

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/url"

	"golang.org/x/oauth2"

	"tg-helper/internal/domains/components/ids"
	"tg-helper/internal/domains/components/oauth"
)

func (s *Service) SetupAuthLink(ctx context.Context, server ids.ServerID, user ids.UserID, accountName, accountDesc string) (*url.URL, error) {
	ctx, span := s.trace.Start(ctx, "Service.SetupAuthLink")
	defer span.End()

	info, err := s.servers.GetServerInfo(ctx, server)
	if err != nil {
		return nil, fmt.Errorf("getting server info: %w", err)
	}

	if info.AuthConfig == nil {
		return nil, ErrAuthUnsupported
	}

	verifier := make([]byte, 32)
	if _, err := rand.Read(verifier); err != nil {
		panic(err)
	}
	verifierStr := base64.RawURLEncoding.EncodeToString(verifier)

	account, err := ids.RandomAccountID(user, server)
	if err != nil {
		return nil, fmt.Errorf("creating random account ID: %w", err)
	}

	state, err := oauth.NewState(
		account,
		accountName,
		accountDesc,
		verifier,
		s.clock().Add(s.stateExpiration),
	)
	if err != nil {
		return nil, fmt.Errorf("creating OAuth state: %w", err)
	}

	return url.Parse(info.AuthConfig.AuthCodeURL(state.State("", s.key), oauth2.S256ChallengeOption(verifierStr)))
}

package accounts

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/url"

	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/oauth"
)

func (s *Usecase) SetupAuthLink(ctx context.Context, server ids.ServerID, user ids.UserID, accountName, accountDesc string) (*url.URL, error) {
	ctx, span := s.trace.Start(ctx, "Service.SetupAuthLink")
	defer span.End()

	info, err := s.servers.GetServerInfo(ctx, server)
	if err != nil {
		return nil, fmt.Errorf("getting server info: %w", err)
	}

	exists, err := s.users.HasUser(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("checking user existence: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("user %q does not exist", user.ID())
	}

	if info.AuthConfig() == nil {
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

	return url.Parse(info.AuthConfig().AuthCodeURL(state.State("", s.key), oauth2.S256ChallengeOption(verifierStr)))
}

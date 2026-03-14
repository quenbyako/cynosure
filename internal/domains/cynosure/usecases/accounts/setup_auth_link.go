package accounts

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/url"
	"time"

	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/oauth"
)

func (s *Usecase) SetupAuthLink(
	ctx context.Context,
	server ids.ServerID,
	user ids.UserID,
	accountName, accountDesc string,
) (
	u *url.URL,
	account ids.AccountID,
	validUntil time.Time,
	err error,
) {
	ctx, span := s.trace.Start(ctx, "Service.SetupAuthLink")
	defer span.End()

	info, err := s.servers.GetServerInfo(ctx, server)
	if err != nil {
		return nil, ids.AccountID{}, time.Time{}, fmt.Errorf("getting server info: %w", err)
	}

	exists, err := s.users.HasUser(ctx, user)
	if err != nil {
		return nil, ids.AccountID{}, time.Time{}, fmt.Errorf("checking user existence: %w", err)
	}

	if !exists {
		return nil, ids.AccountID{}, time.Time{}, fmt.Errorf("user %q does not exist", user.ID())
	}

	if info.AuthConfig() == nil {
		return nil, ids.AccountID{}, time.Time{}, ErrAuthUnsupported
	}

	verifier := make([]byte, 32)
	if _, err := rand.Read(verifier); err != nil {
		panic(err)
	}

	verifierStr := base64.RawURLEncoding.EncodeToString(verifier)

	account, err = ids.RandomAccountID(user, server)
	if err != nil {
		return nil, ids.AccountID{}, time.Time{}, fmt.Errorf("creating random account ID: %w", err)
	}

	validUntil = s.clock().Add(s.stateExpiration)

	state, err := oauth.NewState(
		account,
		accountName,
		accountDesc,
		verifier,
		validUntil,
	)
	if err != nil {
		return nil, ids.AccountID{}, time.Time{}, fmt.Errorf("creating OAuth state: %w", err)
	}

	u, err = url.Parse(info.AuthConfig().AuthCodeURL(state.State("", s.key), oauth2.S256ChallengeOption(verifierStr)))
	if err != nil {
		return nil, ids.AccountID{}, time.Time{}, fmt.Errorf("parsing auth URL: %w", err)
	}

	return u, account, validUntil, nil
}

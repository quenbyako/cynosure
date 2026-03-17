package accounts

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

type SetupAuthLinkResponse struct {
	Link       *url.URL
	ValidUntil time.Time
	Account    ids.AccountID
}

func (s *Usecase) SetupAuthLink(
	ctx context.Context,
	server ids.ServerID,
	user ids.UserID,
	accountName, accountDesc string,
) (SetupAuthLinkResponse, error) {
	ctx, span := s.trace.Start(ctx, "Usecase.SetupAuthLink")
	defer span.End()

	info, err := s.servers.GetServerInfo(ctx, server)
	if err != nil {
		return SetupAuthLinkResponse{}, fmt.Errorf("getting server info: %w", err)
	}

	if err := s.ensureUserExists(ctx, user); err != nil {
		return SetupAuthLinkResponse{}, err
	}

	if info.AuthConfig() == nil {
		return SetupAuthLinkResponse{}, fmt.Errorf("%w", ErrAuthUnsupported)
	}

	return s.createOAuthLink(server, user, accountName, accountDesc, info)
}

func (s *Usecase) ensureUserExists(ctx context.Context, user ids.UserID) error {
	exists, err := s.users.HasUser(ctx, user)
	if err != nil {
		return fmt.Errorf("checking user existence: %w", err)
	}

	if !exists {
		return fmt.Errorf("%w: %q", ErrUserNotFound, user.ID().String())
	}

	return nil
}

func (s *Usecase) createOAuthLink(
	server ids.ServerID,
	user ids.UserID,
	name, desc string,
	info entities.ServerConfigReadOnly,
) (SetupAuthLinkResponse, error) {
	verifier, verifierStr, err := generateVerifier()
	if err != nil {
		return SetupAuthLinkResponse{}, err
	}

	account, err := ids.RandomAccountID(user, server)
	if err != nil {
		return SetupAuthLinkResponse{}, fmt.Errorf("creating randomly: %w", err)
	}

	return s.completeOAuthLink(name, desc, verifier, verifierStr, account, info)
}

func (s *Usecase) completeOAuthLink(
	name, desc string,
	verifier []byte,
	verifierStr string,
	account ids.AccountID,
	info entities.ServerConfigReadOnly,
) (SetupAuthLinkResponse, error) {
	state, validUntil, err := s.generateOAuthState(account, name, desc, verifier)
	if err != nil {
		return SetupAuthLinkResponse{}, err
	}

	authURL, err := url.Parse(info.AuthConfig().AuthCodeURL(
		state,
		oauth2.S256ChallengeOption(verifierStr),
	))
	if err != nil {
		return SetupAuthLinkResponse{}, fmt.Errorf("parsing auth URL: %w", err)
	}

	return SetupAuthLinkResponse{
		Link:       authURL,
		Account:    account,
		ValidUntil: validUntil,
	}, nil
}

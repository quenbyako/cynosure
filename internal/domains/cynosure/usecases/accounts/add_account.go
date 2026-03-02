package accounts

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/oauthhandler"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/oauth"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
	"golang.org/x/oauth2"
)

type AddAccountResponse interface {
	_AddAccountResponse()
}

var _ AddAccountResponse = (*AddAccountResponseAuthRequired)(nil)
var _ AddAccountResponse = (*AddAccountResponseOK)(nil)

type AddAccountResponseAuthRequired struct {
	link       *url.URL
	account    ids.AccountID
	validUntil time.Time
}

func (AddAccountResponseAuthRequired) _AddAccountResponse() {}

func (r AddAccountResponseAuthRequired) AuthURL() *url.URL            { return r.link }
func (r AddAccountResponseAuthRequired) TempAccountID() ids.AccountID { return r.account }
func (r AddAccountResponseAuthRequired) ValidUntil() time.Time        { return r.validUntil }

type AddAccountResponseOK struct {
	account       ids.AccountID
	authAvailable bool
}

func (AddAccountResponseOK) _AddAccountResponse() {}

func (r AddAccountResponseOK) AccountID() ids.AccountID { return r.account }
func (r AddAccountResponseOK) AuthAvailable() bool      { return r.authAvailable }

func (s *Usecase) AddAccount(ctx context.Context, userID ids.UserID, u *url.URL, accName, accDescription string) (AddAccountResponse, error) {
	ctx, span := s.trace.Start(ctx, "AddAccount")
	defer span.End()

	getAccountID := accountBuilder(userID)

	server, err := s.servers.LookupByURL(ctx, u) // just to see if it already exists
	if errors.Is(err, ports.ErrNotFound) {
		var anonTools []tools.RawToolInfo
		server, anonTools, err = s.createServer(ctx, u, getAccountID, accName, accDescription)
		if err != nil {
			return nil, fmt.Errorf("failed to create server: %w", err)
		}
		if len(anonTools) > 0 {
			accountID, err := getAccountID(server.ID())
			if err != nil {
				return nil, fmt.Errorf("failed to create account ID: %w", err)
			}

			account, err := entities.NewAccount(
				accountID,
				accName,
				accDescription,
			)
			if err != nil {
				return nil, fmt.Errorf("constructing account entity: %w", err)
			}

			err = s.accounts.SaveAccount(ctx, account)
			if err != nil {
				return nil, fmt.Errorf("saving account: %w", err)
			}

			if err := s.saveAccountAndTools(ctx, server, account, nil); err != nil {
				return nil, fmt.Errorf("saving account and tools: %w", err)
			}

			return AddAccountResponseOK{account: account.ID(), authAvailable: false}, nil
		}
	} else if err != nil {
		return nil, fmt.Errorf("failed to lookup server: %w", err)
	}

	// found (or discovered) server, let's authorize user.

	if server.AuthConfig() == nil {
		accountID, err := getAccountID(server.ID())
		if err != nil {
			return nil, fmt.Errorf("failed to create account ID: %w", err)
		}

		account, err := entities.NewAccount(
			accountID,
			accName,
			accDescription,
		)
		if err != nil {
			return nil, fmt.Errorf("constructing account entity: %w", err)
		}

		err = s.accounts.SaveAccount(ctx, account)
		if err != nil {
			return nil, fmt.Errorf("saving account: %w", err)
		}

		if err := s.saveAccountAndTools(ctx, server, account, nil); err != nil {
			return nil, fmt.Errorf("saving account and tools: %w", err)
		}

		return AddAccountResponseOK{account: account.ID(), authAvailable: false}, nil
	}

	// next we are going to authorize user.

	exists, err := s.users.HasUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("checking user existence: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("user %q does not exist", userID.ID().String())
	}

	verifier := make([]byte, 32)
	if _, err := rand.Read(verifier); err != nil {
		panic(err)
	}
	verifierStr := base64.RawURLEncoding.EncodeToString(verifier)

	account, err := getAccountID(server.ID())
	if err != nil {
		return nil, fmt.Errorf("creating random account ID: %w", err)
	}

	validUntil := s.clock().Add(s.stateExpiration)

	state, err := oauth.NewState(
		account,
		accName,
		accDescription,
		verifier,
		validUntil,
	)
	if err != nil {
		return nil, fmt.Errorf("creating OAuth state: %w", err)
	}

	u, err = url.Parse(server.AuthConfig().AuthCodeURL(state.State("", s.key), oauth2.S256ChallengeOption(verifierStr)))
	if err != nil {
		return nil, fmt.Errorf("parsing auth URL: %w", err)
	}

	return AddAccountResponseAuthRequired{u, account, validUntil}, nil
}

type makeAccountFunc func(ids.ServerID) (ids.AccountID, error)

func (s *Usecase) createServer(ctx context.Context, u *url.URL, accountIDBuilder makeAccountFunc, slug, description string) (*entities.ServerConfig, []tools.RawToolInfo, error) {
	server, err := entities.NewServerConfig(ids.RandomServerID(), u)
	if err != nil {
		return nil, nil, fmt.Errorf("creating server config: %w", err)
	}

	account, err := accountIDBuilder(server.ID())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create account ID: %w", err)
	}

	// Step 1: Probe MCP server anonymously to check if it's reachable and if it requires auth
	data, mcpErr := s.toolClient.DiscoverTools(ctx, u, account, slug, description)
	if mcpErr == nil {
		if err := s.servers.SetServer(ctx, server); err != nil {
			return nil, nil, fmt.Errorf("failed to set server: %w", err)
		}

		return server, data, nil
	}

	if errors.Is(mcpErr, ports.ErrServerUnreachable) ||
		errors.Is(mcpErr, ports.ErrProtocolNotSupported) {
		// Probably it's some random http server, just forward it.
		return nil, nil, fmt.Errorf("server does not support MCP protocol: %w", mcpErr)
	}

	e := new(ports.RequiresAuthError)
	if !errors.As(mcpErr, &e) {
		return nil, nil, fmt.Errorf("unexpected error while discovering service: %w", mcpErr)
	}
	// gotcha, requires auth. We will try to register OAuth client.

	// Step 2: Try to register OAuth client
	var opts []oauthhandler.RegisterClientOption
	if e.Endpoint() != nil {
		opts = append(opts, oauthhandler.WithSuggestedProtectedResource(e.Endpoint()))
	}

	cfg, expires, oauthErr := s.oauth.RegisterClient(ctx, u, s.oauthClientName, s.oauthRedirectURL)
	if oauthErr != nil {
		return nil, nil, oauthErr
	}

	// Step 3: registering client
	serverOpts := []entities.ServerConfigOption{
		entities.WithAuthConfig(cfg),
	}

	if !expires.IsZero() {
		serverOpts = append(serverOpts, entities.WithExpiration(expires))
	}

	if err := server.SetOAuthConfig(cfg); err != nil {
		return nil, nil, fmt.Errorf("setting OAuth config: %w", err)
	}

	// we created server from complete scratch, so it should not have any events
	server.ClearEvents()
	if err := s.servers.SetServer(ctx, server); err != nil {
		return nil, nil, fmt.Errorf("failed to set server: %w", err)
	}

	return server, nil, nil
}

func accountBuilder(userID ids.UserID) func(ids.ServerID) (ids.AccountID, error) {
	var once sync.Once
	var previousServerID ids.ServerID
	var accountID ids.AccountID
	var err error

	return func(serverID ids.ServerID) (ids.AccountID, error) {
		once.Do(func() {
			previousServerID = serverID
			accountID, err = ids.RandomAccountID(userID, serverID)
		})
		if previousServerID != serverID {
			return ids.AccountID{}, fmt.Errorf("account ID is already set")
		}
		return accountID, err
	}
}

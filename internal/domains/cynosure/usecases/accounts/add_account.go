package accounts

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"

	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/oauthhandler"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/oauth"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
)

type AddAccountResponse interface {
	_AddAccountResponse()
}

var (
	_ AddAccountResponse = (*AddAccountResponseAuthRequired)(nil)
	_ AddAccountResponse = (*AddAccountResponseOK)(nil)
)

type AddAccountResponseAuthRequired struct {
	validUntil time.Time
	link       *url.URL
	account    ids.AccountID
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

// AddAccountByID adds a new account for user by server ID. This might be
// useful, when client already searched this server through API.
//
// This is special case: if user was just created, there must be created a new
// MCP admin account for user.
//
//nolint:ireturn // ReadOnly interface is intentional for domain boundary
func (s *Usecase) AddAccountByID(
	ctx context.Context,
	userID ids.UserID,
	id ids.ServerID,
	accName,
	accDescription string,
) (AddAccountResponse, error) {
	ctx, span := s.trace.Start(ctx, "AddAccountByID")
	defer span.End()

	server, err := s.servers.GetServerInfo(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting server info: %w", err)
	}

	return s.AddAccount(ctx, userID, server.SSELink(), accName, accDescription)
}

//nolint:ireturn // ReadOnly interface is intentional for domain boundary
func (s *Usecase) AddAccount(
	ctx context.Context,
	userID ids.UserID,
	serverURL *url.URL,
	accName,
	accDescription string,
) (AddAccountResponse, error) {
	ctx, span := s.trace.Start(ctx, "AddAccount")
	defer span.End()

	getAccountID := accountBuilder(userID)

	server, err := s.lookupOrCreateServer(ctx, serverURL, getAccountID, accName, accDescription)
	if err != nil {
		return nil, err
	}

	if server.AuthConfig() == nil {
		return s.addAnonymousAccount(ctx, server, getAccountID, accName, accDescription)
	}

	return s.initiateOAuthFlow(ctx, userID, server, getAccountID, accName, accDescription)
}

func (s *Usecase) lookupOrCreateServer(
	ctx context.Context,
	serverURL *url.URL,
	getAccountID makeAccountFunc,
	accName,
	accDescription string,
) (*entities.ServerConfig, error) {
	server, err := s.servers.LookupByURL(ctx, serverURL)
	if err != nil {
		return s.handleLookupError(ctx, serverURL, getAccountID, accName, accDescription, err)
	}

	return server, nil
}

func (s *Usecase) handleLookupError(
	ctx context.Context,
	serverURL *url.URL,
	getAccountID makeAccountFunc,
	accName,
	accDescription string,
	err error,
) (*entities.ServerConfig, error) {
	if !errors.Is(err, ports.ErrNotFound) {
		return nil, fmt.Errorf("lookup server: %w", err)
	}

	var anonTools []tools.RawTool

	server, anonTools, cErr := s.createServer(ctx, serverURL, getAccountID, accName, accDescription)
	if cErr != nil {
		return nil, fmt.Errorf("create server: %w", cErr)
	}

	if len(anonTools) > 0 {
		// handled in addAnonymousAccount
		return server, nil
	}

	return server, nil
}

//nolint:ireturn // ReadOnly interface is intentional for domain boundary
func (s *Usecase) addAnonymousAccount(
	ctx context.Context,
	server entities.ServerConfigReadOnly,
	getAccountID makeAccountFunc,
	accName,
	accDescription string,
) (AddAccountResponse, error) {
	accountID, err := getAccountID(server.ID())
	if err != nil {
		return nil, fmt.Errorf("create account ID: %w", err)
	}

	account, err := entities.NewAccount(accountID, accName, accDescription)
	if err != nil {
		return nil, fmt.Errorf("constructing entity: %w", err)
	}

	if err := s.accounts.SaveAccount(ctx, account); err != nil {
		return nil, fmt.Errorf("saving account: %w", err)
	}

	if err := s.saveAccountAndTools(ctx, server, account, nil); err != nil {
		return nil, fmt.Errorf("saving account and tools: %w", err)
	}

	return AddAccountResponseOK{account: account.ID(), authAvailable: false}, nil
}

//nolint:ireturn // Response interface is intentional
func (s *Usecase) initiateOAuthFlow(
	ctx context.Context,
	userID ids.UserID,
	server entities.ServerConfigReadOnly,
	getAccountID makeAccountFunc,
	accName,
	accDescription string,
) (AddAccountResponse, error) {
	if err := s.checkUserExistence(ctx, userID); err != nil {
		return nil, err
	}

	verifier, verifierStr, err := generateVerifier()
	if err != nil {
		return nil, err
	}

	account, err := getAccountID(server.ID())
	if err != nil {
		return nil, fmt.Errorf("random account ID: %w", err)
	}

	return s.createOAuthAuthResponse(
		account,
		accName,
		accDescription,
		verifier,
		verifierStr,
		server,
	)
}

func (s *Usecase) checkUserExistence(ctx context.Context, userID ids.UserID) error {
	exists, err := s.users.HasUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("checking user existence: %w", err)
	}

	if !exists {
		return fmt.Errorf("%w: %q", ErrUserNotFound, userID.ID().String())
	}

	return nil
}

//nolint:ireturn // Response interface is intentional
func (s *Usecase) createOAuthAuthResponse(
	account ids.AccountID,
	name, description string,
	verifier []byte,
	verifierStr string,
	server entities.ServerConfigReadOnly,
) (AddAccountResponse, error) {
	validUntil := s.clock().Add(s.stateExpiration)

	state, err := oauth.NewState(account, name, description, verifier, validUntil)
	if err != nil {
		return nil, fmt.Errorf("creating state: %w", err)
	}

	authURL, err := url.Parse(server.AuthConfig().AuthCodeURL(
		state.State("", s.key),
		oauth2.S256ChallengeOption(verifierStr),
	))
	if err != nil {
		return nil, fmt.Errorf("parsing auth URL: %w", err)
	}

	return AddAccountResponseAuthRequired{
		link:       authURL,
		account:    account,
		validUntil: validUntil,
	}, nil
}

type makeAccountFunc func(ids.ServerID) (ids.AccountID, error)

func (s *Usecase) createServer(
	ctx context.Context,
	serverAddress *url.URL,
	getAccID makeAccountFunc,
	slug, description string,
) (*entities.ServerConfig, []tools.RawTool, error) {
	server, err := entities.NewServerConfig(ids.RandomServerID(), serverAddress)
	if err != nil {
		return nil, nil, fmt.Errorf("creating config: %w", err)
	}

	account, err := getAccID(server.ID())
	if err != nil {
		return nil, nil, fmt.Errorf("create account ID: %w", err)
	}

	toolList, err := s.toolClient.DiscoverTools(ctx, serverAddress, account, slug, description)
	if err == nil {
		if sErr := s.servers.SetServer(ctx, server); sErr != nil {
			return nil, nil, fmt.Errorf("set server: %w", sErr)
		}

		return server, toolList, nil
	}

	return s.handleDiscoveryError(ctx, server, serverAddress, err)
}

func (s *Usecase) handleDiscoveryError(
	ctx context.Context,
	server *entities.ServerConfig,
	addr *url.URL,
	mcpErr error,
) (*entities.ServerConfig, []tools.RawTool, error) {
	unreach := errors.Is(mcpErr, ports.ErrServerUnreachable)
	proto := errors.Is(mcpErr, ports.ErrProtocolNotSupported)

	if unreach || proto {
		return nil, nil, fmt.Errorf("protocol not supported: %w", mcpErr)
	}

	authErr := new(ports.RequiresAuthError)
	if !errors.As(mcpErr, &authErr) {
		return nil, nil, fmt.Errorf("discovering service: %w", mcpErr)
	}

	return s.registerAndCompleteServer(ctx, server, addr, authErr)
}

func (s *Usecase) registerAndCompleteServer(
	ctx context.Context,
	server *entities.ServerConfig,
	addr *url.URL,
	authErr *ports.RequiresAuthError,
) (*entities.ServerConfig, []tools.RawTool, error) {
	cfg, expires, err := s.oauth.RegisterClient(
		ctx,
		addr,
		s.oauthClientName,
		s.oauthRedirectURL,
		s.getRegisterOptions(authErr)...,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("registering client: %w", err)
	}

	if !expires.IsZero() {
		server.SetConfigExpiration(expires)
	}

	return s.setServerAndSave(ctx, server, cfg)
}

func (s *Usecase) setServerAndSave(
	ctx context.Context,
	server *entities.ServerConfig,
	cfg *oauth2.Config,
) (*entities.ServerConfig, []tools.RawTool, error) {
	if err := server.SetOAuthConfig(cfg); err != nil {
		return nil, nil, fmt.Errorf("setting OAuth config: %w", err)
	}

	server.ClearEvents()

	if err := s.servers.SetServer(ctx, server); err != nil {
		return nil, nil, fmt.Errorf("failed to set server: %w", err)
	}

	return server, nil, nil
}

func (s *Usecase) getRegisterOptions(
	authErr *ports.RequiresAuthError,
) []oauthhandler.RegisterClientOption {
	var opts []oauthhandler.RegisterClientOption

	if authErr.Endpoint() != nil {
		opts = append(opts, oauthhandler.WithSuggestedProtectedResource(authErr.Endpoint()))
	}

	return opts
}

func accountBuilder(userID ids.UserID) makeAccountFunc {
	var (
		once             sync.Once
		previousServerID ids.ServerID
		accountID        ids.AccountID
		err              error
	)

	return func(serverID ids.ServerID) (ids.AccountID, error) {
		once.Do(func() {
			previousServerID = serverID
			accountID, err = ids.RandomAccountID(userID, serverID)
		})

		if previousServerID != serverID {
			return ids.AccountID{}, fmt.Errorf("%w", ErrAccountIDAlreadySet)
		}

		if err != nil {
			return ids.AccountID{}, fmt.Errorf("random account ID: %w", err)
		}

		return accountID, nil
	}
}

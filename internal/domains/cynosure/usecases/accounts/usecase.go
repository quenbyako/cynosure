package accounts

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/quenbyako/cynosure/contrib/taskpool"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/accounts"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/embedding"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/identitymanager"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/oauthhandler"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/ratelimiter"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/toolclient"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/oauth"
)

//nolint:lll // makes no sense actually.
var (
	ErrAuthUnsupported = errors.New("authorization for this server is not supported, allowed to connect anonymously")
)

type Usecase struct {
	oauth            oauthhandler.Port
	trace            trace.Tracer
	accounts         accounts.Port
	tools            ports.ToolStorage
	index            embedding.Port
	limiter          ratelimiter.Port
	toolClient       toolclient.Port
	servers          ports.ServerStorage
	users            identitymanager.Port
	clock            func() time.Time
	pool             *taskpool.TaskPool[discoveryTask]
	log              LogCallbacks
	oauthRedirectURL *url.URL
	oauthClientName  string
	stateExpiration  time.Duration
	key              [16]byte
}

type discoveryTask struct {
	server  entities.ServerConfigReadOnly
	account entities.AccountReadOnly
	token   *oauth2.Token
}

type newParams struct {
	tracer           trace.TracerProvider
	oauthRedirectURL *url.URL
	clientName       string
	stateExpiration  time.Duration
	fixedKey         [16]byte
}

type NewOption func(*newParams)

func WithOAuthClientName(name string) NewOption {
	return func(p *newParams) { p.clientName = name }
}

func WithFixedKey(key [16]byte) NewOption {
	return func(p *newParams) { p.fixedKey = key }
}

func WithStateExpiration(d time.Duration) NewOption {
	return func(p *newParams) { p.stateExpiration = d }
}

func WithOAuthRedirectURL(u *url.URL) NewOption {
	return func(p *newParams) { p.oauthRedirectURL = u }
}

func WithTracerProvider(tp trace.TracerProvider) NewOption {
	return func(p *newParams) { p.tracer = tp }
}

func New(
	servers ports.ServerStorage,
	authHandler oauthhandler.Port,
	accounts accounts.Port,
	tools ports.ToolStorage,
	index embedding.Port,
	toolClient toolclient.Port,
	users identitymanager.Port,
	limiter ratelimiter.Port,
	opts ...NewOption,
) (*Usecase, error) {
	params := buildNewParams(opts...)

	usecase := newUsecase(
		servers,
		authHandler,
		accounts,
		tools,
		index,
		toolClient,
		users,
		limiter,
		&params,
	)

	if err := usecase.validate(); err != nil {
		return nil, fmt.Errorf("usecase validation: %w", err)
	}

	return usecase, nil
}

func newUsecase(
	servers ports.ServerStorage, authHandler oauthhandler.Port, accounts accounts.Port,
	tools ports.ToolStorage, index embedding.Port, toolClient toolclient.Port,
	users identitymanager.Port, limiter ratelimiter.Port, params *newParams,
) *Usecase {
	usecase := &Usecase{
		toolClient: toolClient, oauth: authHandler, servers: servers,
		accounts: accounts, tools: tools, index: index, limiter: limiter,
		users: users, clock: time.Now, log: NoOpLogCallbacks{},
		oauthRedirectURL: params.oauthRedirectURL, oauthClientName: params.clientName,
		key: params.fixedKey, stateExpiration: params.stateExpiration,
		trace: params.tracer.Tracer(pkgName),
		pool:  nil, // set below
	}

	usecase.pool = taskpool.New(discoveryPoolWorkers, usecase.runDiscoveryTask)

	return usecase
}

func (s *Usecase) Run(ctx context.Context) error {
	if err := s.pool.Run(ctx); err != nil {
		return fmt.Errorf("running usecase task pool: %w", err)
	}

	return nil
}

func buildNewParams(opts ...NewOption) newParams {
	params := newParams{
		clientName:       "test-client",
		fixedKey:         randomAuthKey(),
		stateExpiration:  stateExpiration,
		tracer:           noop.NewTracerProvider(),
		oauthRedirectURL: nil,
	}

	for _, opt := range opts {
		opt(&params)
	}

	return params
}

func (s *Usecase) validate() error {
	if err := s.validatePorts(); err != nil {
		return err
	}

	return s.validateConfig()
}

func (s *Usecase) validatePorts() error {
	if err := s.validateStoragePorts(); err != nil {
		return err
	}

	return s.validateLogicPorts()
}

func (s *Usecase) validateStoragePorts() error {
	switch {
	case s.servers == nil:
		return ErrInternalValidation("server storage is required")
	case s.accounts == nil:
		return ErrInternalValidation("account storage is required")
	case s.tools == nil:
		return ErrInternalValidation("tool storage is required")
	case s.users == nil:
		return ErrInternalValidation("user storage is required")
	default:
		return nil
	}
}

func (s *Usecase) validateLogicPorts() error {
	switch {
	case s.toolClient == nil:
		return ErrInternalValidation("tool registry is required")
	case s.oauth == nil:
		return ErrInternalValidation("OAuth handler is required")
	case s.index == nil:
		return ErrInternalValidation("tool semantic index is required")
	case s.limiter == nil:
		return ErrInternalValidation("rate limiter is required")
	default:
		return nil
	}
}

func (s *Usecase) validateConfig() error {
	if s.oauthRedirectURL == nil {
		return ErrInternalValidation("OAuth redirect URL is required")
	}

	if s.oauthClientName == "" {
		return ErrInternalValidation("OAuth client name is required")
	}

	if s.key == [16]byte{} {
		return ErrInternalValidation("OAuth key is required")
	}

	if s.stateExpiration == 0 {
		return ErrInternalValidation("state expiration is required")
	}

	return nil
}

func (s *Usecase) GetServerInfo(
	ctx context.Context,
	id ids.ServerID,
) (entities.ServerConfigReadOnly, error) {
	info, err := s.servers.GetServerInfo(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting server info: %w", err)
	}

	return info, nil
}

func randomAuthKey() [16]byte {
	var key [16]byte

	if _, err := rand.Read(key[:]); err != nil {
		//nolint:forbidigo // system-wide failure, absolutely unsafe to ignore
		panic(err)
	}

	return key
}

func generateVerifier() (verifier []byte, verifierStr string, err error) {
	verifier = make([]byte, sha256.Size)
	if _, err = rand.Read(verifier); err != nil {
		return nil, "", fmt.Errorf("failed to generate verifier: %w", err)
	}

	return verifier, base64.RawURLEncoding.EncodeToString(verifier), nil
}

func (s *Usecase) generateOAuthState(
	account ids.AccountID, name, desc string, verifier []byte,
) (string, time.Time, error) {
	validUntil := s.clock().Add(s.stateExpiration)

	stateRaw, err := oauth.NewState(account, name, desc, verifier, validUntil)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("creating state: %w", err)
	}

	state, err := stateRaw.State("", s.key)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("generating state: %w", err)
	}

	return state, validUntil, nil
}

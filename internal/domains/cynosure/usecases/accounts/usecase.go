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

	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/identitymanager"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/oauthhandler"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/toolclient"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

//nolint:lll // makes no sense actually.
var (
	ErrAuthUnsupported = errors.New("authorization for this server is not supported, allowed to connect anonymously")
)

type Usecase struct {
	users            identitymanager.Port
	servers          ports.ServerStorage
	accounts         ports.AccountStorage
	tools            ports.ToolStorage
	index            ports.ToolSemanticIndex
	toolClient       toolclient.Port
	oauth            oauthhandler.Port
	trace            trace.Tracer
	clock            func() time.Time
	oauthRedirectURL *url.URL
	oauthClientName  string
	stateExpiration  time.Duration
	key              [16]byte
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

const (
	stateExpiration = 5 * time.Minute
)

func New(
	servers ports.ServerStorage,
	oauth oauthhandler.Port,
	accounts ports.AccountStorage,
	tools ports.ToolStorage,
	index ports.ToolSemanticIndex,
	toolClient toolclient.Port,
	users identitymanager.Port,
	opts ...NewOption,
) (*Usecase, error) {
	params := buildNewParams(opts...)

	usecase := newUsecase(servers, oauth, accounts, tools, index, toolClient, users, &params)

	if err := usecase.validate(); err != nil {
		return nil, fmt.Errorf("usecase validation: %w", err)
	}

	return usecase, nil
}

func newUsecase(
	servers ports.ServerStorage,
	oauth oauthhandler.Port,
	accounts ports.AccountStorage,
	tools ports.ToolStorage,
	index ports.ToolSemanticIndex,
	toolClient toolclient.Port,
	users identitymanager.Port,
	params *newParams,
) *Usecase {
	return &Usecase{
		toolClient: toolClient,
		oauth:      oauth,
		servers:    servers,
		accounts:   accounts,
		tools:      tools,
		index:      index,
		users:      users,
		clock:      time.Now,

		oauthRedirectURL: params.oauthRedirectURL,
		oauthClientName:  params.clientName,
		key:              params.fixedKey,
		stateExpiration:  params.stateExpiration,

		trace: params.tracer.Tracer(pkgName),
	}
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
	if s.toolClient == nil {
		return ErrInternalValidation("tool registry is required")
	}

	if s.servers == nil {
		return ErrInternalValidation("server storage is required")
	}

	if s.oauth == nil {
		return ErrInternalValidation("OAuth handler is required")
	}

	if s.accounts == nil {
		return ErrInternalValidation("account storage is required")
	}

	if s.tools == nil {
		return ErrInternalValidation("tool storage is required")
	}

	if s.index == nil {
		return ErrInternalValidation("tool semantic index is required")
	}

	if s.users == nil {
		return ErrInternalValidation("user storage is required")
	}

	return nil
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

//nolint:ireturn // ReadOnly interface is intentional for domain boundary
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

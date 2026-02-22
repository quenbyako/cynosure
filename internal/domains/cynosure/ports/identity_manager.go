package ports

import (
	"context"

	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

// IdentityManager manages user persistence and identity mapping.
type IdentityManager interface {
	IdentityManagerRead
	IdentityManagerWrite
}

// IdentityManagerRead defines read operations for user identities.
type IdentityManagerRead interface {
	// HasUser checks if a user with the given ID exists in the system.
	//
	// See next test suites to find how it works:
	//
	//  - [TestIdentityManager] — verifying existence of saved users.
	//
	// Throws:
	//
	//  - [ErrNotFound]: if the user does not exist.
	HasUser(ctx context.Context, id ids.UserID) (bool, error)

	// LookupUser retrieves a user ID by its external provider and ID.
	//
	// See next test suites to find how it works:
	//
	//  - [TestIdentityManagerLookup] — looking up users by external identity.
	//
	// Throws:
	//
	//  - [ErrNotFound]: if no mapping exists for the given provider and external ID.
	LookupUser(ctx context.Context, telegramID string) (ids.UserID, error)
}

// IdentityManagerWrite defines write operations for user identities.
type IdentityManagerWrite interface {
	// CreateUser creates a new user, pushes it to external identity system if
	// necessary, and persists locally. Returns the newly created user ID.
	//
	// TODO: currently only one provider supported — Telegram. It's strictly
	// necessary to add primitives (or entites?) for each user.
	//
	// See next test suites to find how it works:
	//
	//  - [TestCreateUser] — verifying new user creation and data persistence.
	//
	// Throws:
	//
	//  - [ErrAlreadyExists]: if a user with such parameters already exists.
	CreateUser(ctx context.Context, telegramID, nickname, firstName, lastName string) (ids.UserID, error)

	// SaveUserMapping persists a mapping between an internal user ID and
	// an external provider's ID.
	//
	// See next test suites to find how it works:
	//
	//  - [TestSaveUserMapping] — verifying mapping persistence.
	//
	// Throws:
	//
	//  - [ErrAlreadyExists]: if the mapping or user ID already exists.
	SaveUserMapping(ctx context.Context, id ids.UserID, provider, externalID string) error
}

type IdentityManagerFactory interface {
	IdentityManager() IdentityManagerWrapped
}

func NewIdentityManager(factory IdentityManagerFactory) IdentityManagerWrapped {
	return factory.IdentityManager()
}

type IdentityManagerWrapped interface {
	IdentityManager

	_IdentityManager()
}

type identityManagerWrapped struct {
	w IdentityManager

	trace trace.Tracer
}

var _ IdentityManager = (*identityManagerWrapped)(nil)

func (i *identityManagerWrapped) _IdentityManager() {}

type WrapIdentityManagerOption func(*identityManagerWrapped)

// WithIdentityManagerTrace expects initialized tracer, cause traces must show
// REAL package name, instead of wrapper.
func WithIdentityManagerTrace(trace trace.Tracer) WrapIdentityManagerOption {
	return func(p *identityManagerWrapped) { p.trace = trace }
}

func WrapIdentityManager(storage IdentityManager, opts ...WrapIdentityManagerOption) IdentityManagerWrapped {
	i := identityManagerWrapped{
		w:     storage,
		trace: noop.NewTracerProvider().Tracer(""),
	}
	for _, opt := range opts {
		opt(&i)
	}

	return &i
}

func (i *identityManagerWrapped) HasUser(ctx context.Context, id ids.UserID) (bool, error) {
	ctx, span := i.trace.Start(ctx, "HasUser")
	defer span.End()

	res, err := i.w.HasUser(ctx, id)
	if err != nil {
		span.RecordError(err)
	}

	return res, err
}

func (i *identityManagerWrapped) LookupUser(ctx context.Context, telegramID string) (ids.UserID, error) {
	ctx, span := i.trace.Start(ctx, "LookupUser")
	defer span.End()

	res, err := i.w.LookupUser(ctx, telegramID)
	if err != nil {
		span.RecordError(err)
	}

	return res, err
}

func (i *identityManagerWrapped) CreateUser(ctx context.Context, telegramID, nickname, firstName, lastName string) (ids.UserID, error) {
	ctx, span := i.trace.Start(ctx, "CreateUser")
	defer span.End()

	res, err := i.w.CreateUser(ctx, telegramID, nickname, firstName, lastName)
	if err != nil {
		span.RecordError(err)
	}

	return res, err
}

func (i *identityManagerWrapped) SaveUserMapping(ctx context.Context, id ids.UserID, provider, externalID string) error {
	ctx, span := i.trace.Start(ctx, "SaveUserMapping")
	defer span.End()

	err := i.w.SaveUserMapping(ctx, id, provider, externalID)
	if err != nil {
		span.RecordError(err)
	}

	return err
}

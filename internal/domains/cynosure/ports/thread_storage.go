package ports

import (
	"context"

	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

// ThreadStorage manages persistence of conversation threads (chat history).
// Each thread is an aggregate containing an ordered sequence of messages from
// user, assistant, and tool interactions. Implements Optimistic Concurrency
// Control via position-based versioning.
type ThreadStorage interface {
	// CreateThread initializes a new conversation thread. Typically called once
	// when starting a new chat session. Thread starts with zero messages.
	//
	// See next test suites to find how it works:
	//
	//  - [TestCreateThread] — creating new threads and verifying initial state
	CreateThread(ctx context.Context, thread entities.ThreadReadOnly) error

	// GetThread retrieves thread with all messages in chronological order.
	// Performs eager loading of complete message history.
	//
	// See next test suites to find how it works:
	//
	//  - [TestGetThread] — retrieving thread with messages
	//
	// Throws:
	//
	//  - [ErrNotFound] if thread doesn't exist.
	GetThread(ctx context.Context, threadID ids.ThreadID) (*entities.Thread, error)

	// UpdateThread persists pending messages from thread's event stream.
	// Implements Optimistic Concurrency Control - fails if thread was modified
	// concurrently (detects conflicts via last_message_pos). Does not validate
	// message content or tool arguments - validation happens in domain layer.
	//
	// See next test suites to find how it works:
	//
	//  - [TestUpdateThread] — persisting messages with OCC conflict detection
	//  - [TestConcurrentModification] — verifying OCC prevents data loss
	UpdateThread(ctx context.Context, thread entities.ThreadReadOnly) error
}

type ThreadStorageFactory interface {
	ThreadStorage() ThreadStorageWrapped
}

func NewThreadStorage(factory ThreadStorageFactory) ThreadStorageWrapped {
	return factory.ThreadStorage()
}

type ThreadStorageWrapped interface {
	ThreadStorage

	_ThreadStorage()
}

type threadStorageWrapped struct {
	w ThreadStorage

	trace trace.Tracer
}

func (t *threadStorageWrapped) _ThreadStorage() {}

type WrapThreadStorageOption func(*threadStorageWrapped)

// Unlike common option that expects TracerProvider, this option expects
// initialized tracer, cause traces must show REAL package name, instead of
// wrapper.
func WithTrace(trace trace.Tracer) WrapThreadStorageOption {
	return func(p *threadStorageWrapped) { p.trace = trace }
}

func WrapThreadStorage(storage ThreadStorage, opts ...WrapThreadStorageOption) ThreadStorageWrapped {
	t := threadStorageWrapped{
		w:     storage,
		trace: noop.NewTracerProvider().Tracer(""),
	}
	for _, opt := range opts {
		opt(&t)
	}

	return &t
}

func (t *threadStorageWrapped) CreateThread(ctx context.Context, thread entities.ThreadReadOnly) error {
	ctx, span := t.trace.Start(ctx, "CreateThread")
	defer span.End()

	err := t.w.CreateThread(ctx, thread)
	if err != nil {
		span.RecordError(err)
	}

	return err
}

func (t *threadStorageWrapped) GetThread(ctx context.Context, threadID ids.ThreadID) (*entities.Thread, error) {
	ctx, span := t.trace.Start(ctx, "GetThread")
	defer span.End()

	res, err := t.w.GetThread(ctx, threadID)
	if err != nil {
		span.RecordError(err)
	}

	return res, err
}

func (t *threadStorageWrapped) UpdateThread(ctx context.Context, thread entities.ThreadReadOnly) error {
	ctx, span := t.trace.Start(ctx, "UpdateThread")
	defer span.End()

	err := t.w.UpdateThread(ctx, thread)
	if err != nil {
		span.RecordError(err)
	}

	return err
}

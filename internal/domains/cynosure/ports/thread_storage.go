package ports

import (
	"context"

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

	// SaveThread persists pending messages from thread's event stream.
	// Implements Optimistic Concurrency Control - fails if thread was modified
	// concurrently (detects conflicts via last_message_pos). Does not validate
	// message content or tool arguments - validation happens in domain layer.
	//
	// See next test suites to find how it works:
	//
	//  - [TestSaveThread] — persisting messages with OCC conflict detection
	//  - [TestConcurrentModification] — verifying OCC prevents data loss
	SaveThread(ctx context.Context, thread entities.ThreadReadOnly) error
}

type ThreadStorageFactory interface {
	ThreadStorage() ThreadStorage
}

func NewThreadStorage(factory ThreadStorageFactory) ThreadStorage {
	return factory.ThreadStorage()
}

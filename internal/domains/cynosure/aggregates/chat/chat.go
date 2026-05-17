// Package chat implements chat aggregate.
package chat

import (
	"context"
	"fmt"
	"sync"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/embedding"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
)

// EmbeddingProvider is a callback that lazy-evaluates tool embeddings for the
// given context.
type EmbeddingProvider func(
	ctx context.Context,
	user ids.UserID,
	messages []messages.Message,
) ([embedding.EmbeddingSize]float32, error)

// Chat is the Aggregate Root for a conversation session.
//
// Architectural Role:
// The Chat aggregate is responsible for maintaining the consistency and integrity
// of a conversation. It acts as a transactional boundary for state changes.
//
// Key Responsibilities:
//  1. **State Management**: Holds the authoritative history of messages (`ChatHistory`).
//  2. **Context Management**: Determines which tools are statistically relevant
//     for the current conversation state (RAG). This is critical for managing
//     the LLM's limited context window.
//  3. **Atomicity**: Ensures that every state transition (adding a message) is
//     either fully persisted or fully rolled back. This includes synchronizing
//     in-memory state (history + tool context) with persistent storage.
//
// Invariants:
//   - The list of available tools (`reversedTools`) MUST always correspond to the
//     state of the conversation *after* the last user message.
//   - Messages MUST be strictly ordered.
//
// Why not just a Service?
// A Service would typically coordinate stateless operations. However, a Chat
// has complex internal state (history + context) that needs to evolve together.
// Moving this logic to a Service would lead to anemic domain models where the
// Service manually juggles history updates and RAG calls, increasing the risk
// of inconsistent state (e.g., history updated but tools not refreshed).
type Chat struct {
	storage           ports.ThreadStorage
	embeddingProvider EmbeddingProvider
	toolStorage       ports.ToolStorage
	accounts          ports.AccountStorage
	thread            *entities.Thread
	// tools caches the actual entities.Tool objects for execution after model
	// picks them
	tools map[ids.ToolID]*entities.Tool
	// list of active tool calls that didn't get response yet.
	activeCalls map[string]struct{}

	// Immutable collection of relevant tools
	//
	// If toolbox is invalid, it indicates that should be recreated.
	toolbox             tools.Toolbox
	toolboxContextLimit uint
	mu                  sync.RWMutex
}

func New(
	ctx context.Context,
	storage ports.ThreadStorage,
	embeddingProvider EmbeddingProvider,
	toolStorage ports.ToolStorage,
	accounts ports.AccountStorage,
	threadID ids.ThreadID,
	toolboxContextLimit uint,
) (*Chat, error) {
	thread, err := storage.GetThread(ctx, threadID)
	if err != nil {
		return nil, fmt.Errorf("getting thread: %w", err)
	}

	return newChatAggregate(
		thread, storage, embeddingProvider, toolStorage, accounts, toolboxContextLimit,
	)
}

func CreateChatAggregate(
	ctx context.Context,
	storage ports.ThreadStorage,
	embeddingProvider EmbeddingProvider,
	toolStorage ports.ToolStorage,
	accounts ports.AccountStorage,
	threadID ids.ThreadID,
	history []messages.Message,
	toolboxContextLimit uint,
) (*Chat, error) {
	thread, err := entities.NewThread(threadID, history)
	if err != nil {
		return nil, fmt.Errorf("creating thread: %w", err)
	}

	err = storage.CreateThread(ctx, thread)
	if err != nil {
		return nil, fmt.Errorf("creating thread in storage: %w", err)
	}

	return newChatAggregate(
		thread, storage, embeddingProvider, toolStorage, accounts, toolboxContextLimit,
	)
}

func newChatAggregate(
	thread *entities.Thread,
	storage ports.ThreadStorage,
	embeddingProvider EmbeddingProvider,
	toolStorage ports.ToolStorage,
	accounts ports.AccountStorage,
	toolboxContextLimit uint,
) (*Chat, error) {
	chat := initChat(
		thread,
		storage,
		embeddingProvider,
		toolStorage,
		accounts,
		toolboxContextLimit,
	)
	if err := chat.validate(); err != nil {
		return nil, err
	}

	// We no longer build the toolbox on initialization.
	// It is built lazily on the first call to RelevantTools().
	return chat, nil
}

func initChat(
	thread *entities.Thread,
	storage ports.ThreadStorage,
	embeddingProvider EmbeddingProvider,
	toolStorage ports.ToolStorage,
	accounts ports.AccountStorage,
	toolboxContextLimit uint,
) *Chat {
	return &Chat{
		thread:      thread,
		toolbox:     tools.Toolbox{}, // setting invalid toolbox forces recreation on first call
		tools:       make(map[ids.ToolID]*entities.Tool),
		activeCalls: make(map[string]struct{}),

		storage:             storage,
		embeddingProvider:   embeddingProvider,
		toolStorage:         toolStorage,
		accounts:            accounts,
		toolboxContextLimit: toolboxContextLimit,

		mu: sync.RWMutex{},
	}
}

func (c *Chat) validate() error {
	if c.storage == nil {
		return errInternalValidation("storage is nil")
	}

	if c.embeddingProvider == nil {
		return errInternalValidation("embeddingProvider is nil")
	}

	if c.toolStorage == nil {
		return errInternalValidation("toolStorage is nil")
	}

	if c.accounts == nil {
		return errInternalValidation("accounts is nil")
	}

	if !c.thread.Valid() {
		return errInternalValidation("userthread is invalid")
	}

	return nil
}

// RelevantTools returns the current toolbox containing all relevant tools for
// this conversation. The toolbox is rebuilt after each user message based on
// semantic search.
func (c *Chat) RelevantTools(ctx context.Context) (toolbox tools.Toolbox, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.toolbox.Valid() {
		return c.toolbox, nil
	}

	if c.toolbox, err = c.buildToolbox(ctx, c.toolboxContextLimit); err != nil {
		return tools.Toolbox{}, err
	}

	return c.toolbox, nil
}

func (c *Chat) ThreadID() ids.ThreadID {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.thread.ID()
}

func (c *Chat) Messages(limit uint) []messages.Message {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.thread.Messages(limit)
}

func (c *Chat) AgentID() ids.AgentID {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.thread.AgentID()
}

func (c *Chat) SetAgent(ctx context.Context, agentID ids.AgentID) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.thread.SetAgent(agentID) {
		return nil
	}

	if err := c.storage.UpdateThread(ctx, c.thread); err != nil {
		return fmt.Errorf("saving thread with new agent: %w", err)
	}

	c.thread.ClearEvents()

	return nil
}

// SetToolboxContext updates the limit of messages to be used for building the
// toolbox. This is useful for agents that need to change the context limit
// dynamically.
//
// This limit is ephemeral (DOES NOT save in any storage) and will be reset to
// the default value after the next chat aggregate recreaction.
func (c *Chat) SetToolboxContext(limit uint) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.toolboxContextLimit = limit
	c.toolbox = tools.Toolbox{}
}

func (c *Chat) ToolboxContext() uint {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.toolboxContextLimit
}

package ports

import (
	"context"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

// AgentStorage manages persistence of AI agent configurations (model settings).
// Each agent defines the LLM model, system prompt, sampling parameters
// (temperature, topP), and stop words for conversation generation.
type AgentStorage interface {
	AgentStorageRead
	AgentStorageWrite
}

type AgentStorageFactory interface {
	AgentStorage() AgentStorage
}

//nolint:ireturn // standard port pattern: hiding implementation details
func NewAgentStorage(factory AgentStorageFactory) AgentStorage {
	return factory.AgentStorage()
}

type AgentStorageRead interface {
	// ListAgents returns all agent configurations owned by the user. Empty
	// slice if none exist. Includes full agent details (not just IDs).
	//
	// See next test suites to find how it works:
	//
	//  -  [TestSaveModel] — listing agents and verifying saved agent is present
	ListAgents(ctx context.Context, user ids.UserID) ([]*entities.Agent, error)

	// GetAgent retrieves full agent configuration by ID.
	//
	// See next test suites to find how it works:
	//
	//  -  [TestSaveModel] — retrieving agent and verifying all fields match
	//     saved values
	//
	// Throws:
	//
	//  - [ErrNotFound] if agent doesn't exist.
	GetAgent(ctx context.Context, model ids.AgentID) (*entities.Agent, error)
}

type AgentStorageWrite interface {
	// SaveAgent creates or updates agent configuration (upsert). Does not
	// validate model name or parameter ranges - validation happens in domain
	// layer.
	//
	// See next test suites to find how it works:
	//
	//  -  [TestSaveModel] — persisting agent configuration and retrieving it
	//     back
	SaveAgent(ctx context.Context, model entities.AgentReadOnly) error

	// DeleteAgent permanently removes agent configuration. Idempotent
	// operation. Does not cascade to threads using this agent - threads retain
	// historical agent_id references.
	//
	// See next test suites to find how it works:
	//
	//  -  [TestSaveModel] — deleting agent and verifying it's no longer
	//     retrievable
	//
	// Throws:
	//
	//  - [ErrNotFound] if agent doesn't exist (implementations may ignore).
	DeleteAgent(ctx context.Context, model ids.AgentID) error
}

package ports

import (
	"context"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

const embeddingSize = 1536

// ToolStorage manages persistence of MCP tools with their semantic embeddings.
// Each tool represents a callable function exposed by an MCP server account,
// stored with vector embeddings for semantic search capabilities.
type ToolStorage interface {
	ToolStorageRead
	ToolStorageWrite
}

type ToolStorageRead interface {
	// ListTools returns all tools for a specific account. Empty slice if none
	// exist. Does not perform semantic filtering - returns raw list.
	//
	// See next test suites to find how it works:
	//
	//  - [TestListTools] — listing tools for specific account
	ListTools(ctx context.Context, account ids.AccountID) ([]*entities.Tool, error)

	// GetTool retrieves a specific tool by account and tool ID.
	//
	// See next test suites to find how it works:
	//
	//  - [TestGetTool] — retrieving specific tool by account and ID
	//
	// Throws:
	//
	//  - [ErrNotFound] if tool doesn't exist.
	GetTool(ctx context.Context, account ids.AccountID, tool ids.ToolID) (*entities.Tool, error)

	// LookupTools performs semantic search for relevant tools using vector
	// similarity. Returns top-K tools ranked by cosine similarity to query
	// embedding. Empty result is not an error - returns empty slice.
	//
	// See next test suites to find how it works:
	//
	//  - [TestToolEmbeddingSearch] — semantic search with various query types
	//
	// Parameters:
	//  - embedding: Query vector from ToolSemanticIndex.BuildToolEmbedding
	//  - limit: Maximum number of results (top-K)
	LookupTools(ctx context.Context, user ids.UserID, embedding [embeddingSize]float32, limit int) ([]*entities.Tool, error)
}

type ToolStorageWrite interface {
	// SaveTool creates or updates tool with its embedding (upsert). Tool must
	// belong to an existing account. Does not validate tool schema correctness
	// - validation happens in domain layer.
	//
	// See next test suites to find how it works:
	//
	//  - [TestSaveTool] — persisting tools with embeddings
	SaveTool(ctx context.Context, info entities.ToolReadOnly) error

	// DeleteTool removes tool by ID. Idempotent operation. Does not cascade to
	// threads or messages that referenced this tool - historical tool calls
	// retain their references.
	//
	// See next test suites to find how it works:
	//
	//  - [TestDeleteTool] — removing tools and verifying cascade behavior
	DeleteTool(ctx context.Context, tool ids.ToolID) error
}

type ToolStorageFactory interface {
	ToolStorage() ToolStorage
}

func NewToolStorage(f ToolStorageFactory) ToolStorage {
	return f.ToolStorage()
}

package ports

import (
	"context"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

// ToolSemanticIndex generates embeddings for RAG-based tool discovery.
// Works as the first stage in a three-stage workflow:
//  1. ToolSemanticIndex generates embeddings from conversation context
//  2. ToolStorage performs vector similarity search
//  3. Chat aggregate builds Toolbox from retrieved tools
//
// This separation enables swapping embedding providers and vector databases
// independently without affecting each other.
type ToolSemanticIndex interface {
	// IndexTool generates and returns semantic embedding for a tool. Called
	// after tool registration to make tools discoverable via semantic search.
	//
	// See next test suites to find how it works:
	//
	//  - [TestToolEmbeddingGeneration] — embedding generation for various tool
	//     types
	//
	IndexTool(ctx context.Context, tool entities.ToolReadOnly) ([embeddingSize]float32, error)

	// BuildToolEmbedding generates query embedding from conversation context.
	// Used to find semantically relevant tools matching user's intent.
	//
	// See next test suites to find how it works:
	//
	//  - [TestToolEmbeddingSearch] — semantic search with various conversation
	//     contexts
	//
	BuildToolEmbedding(ctx context.Context, msgs []messages.Message) ([embeddingSize]float32, error)
}

type ToolSemanticIndexFactory interface {
	ToolSemanticIndex() ToolSemanticIndex
}

//nolint:ireturn // standard port pattern: hiding implementation details
func NewToolSemanticIndex(f ToolSemanticIndexFactory) ToolSemanticIndex {
	return f.ToolSemanticIndex()
}

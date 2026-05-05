package ports

import (
	"context"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

// ToolSemanticIndex generates embeddings for RAG-based tool discovery. Works as
// the first stage in a three-stage workflow:
//
//  1. ToolSemanticIndex generates embeddings from conversation context
//  2. ToolStorage performs vector similarity search
//  3. Chat aggregate builds Toolbox from retrieved tools
//
// This separation enables swapping embedding providers and vector databases
// independently without affecting each other.
//
// # Rate Limiting
//
// This port is subject to usage quotas. To prevent over usage of expensive API,
// each adapter may implement (or may not!) its own hard caps for usage. This
// behavior is a fallback, if domain layer of quota management will fail to do
// its job, e.g. if too many users will try to use the servise, global quota per
// each module may trigger by throwing [HardQuotaExhaustedError]. Note, that
// this error may happen only BEFORE request will be sent to the provider, it
// doesn't support ratelimit debt.
type ToolSemanticIndex interface {
	// IndexTool generates and returns semantic embedding for a tool. Called
	// after tool registration to make tools discoverable via semantic search.
	//
	// See next test suites to find how it works:
	//
	//  - [TestToolEmbeddingGeneration] — embedding generation for various tool
	//     types
	//
	IndexTool(ctx context.Context, tool entities.ToolReadOnly) ([EmbeddingSize]float32, error)

	// BuildToolEmbedding generates query embedding from conversation context.
	// Used to find semantically relevant tools matching user's intent.
	//
	// See next test suites to find how it works:
	//
	//  - [TestToolEmbeddingSearch] — semantic search with various conversation
	//     contexts
	//
	// Throws:
	//
	//  - [HardQuotaExhaustedError] if hard quota is exhausted. See
	//    documentation for [ToolSemanticIndex] to get more about quotas and
	//    rate limiting.
	BuildToolEmbedding(ctx context.Context, msgs []messages.Message) ([EmbeddingSize]float32, error)
}

type ToolSemanticIndexFactory interface {
	ToolSemanticIndex() ToolSemanticIndex
}

func NewToolSemanticIndex(f ToolSemanticIndexFactory) ToolSemanticIndex {
	return f.ToolSemanticIndex()
}

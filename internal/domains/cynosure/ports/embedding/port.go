package embedding

import (
	"context"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

const (
	EmbeddingSize = 1536
)

// Port generates embeddings for RAG-based tool discovery. Works as
// the first stage in a three-stage workflow:
//
//  1. Embedding port generates embeddings from conversation context
//  2. ToolStorage Port performs vector similarity search
//  3. Chat Port aggregate builds Toolbox from retrieved tools
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
type Port interface {
	// IndexTool generates and returns semantic embedding for a tool. Called
	// after tool registration to make tools discoverable via semantic search.
	//
	// See next test suites to find how it works:
	//
	//  - [TestToolEmbeddingGeneration] — embedding generation for various tool
	//     types
	//
	// Throws:
	//
	//  - [ErrHardQuotaExhausted] if hard quota is exhausted. See
	//    documentation for [Port] to get more about quotas and
	//    rate limiting.
	//  - [PreflightFailedError] if preflight check fails. Note, that this error
	//    interprets as a business logic error, and if preflight failed due to
	//    infrastructure issues, callback is responsible to handle observability
	//    features.
	IndexTool(
		ctx context.Context,
		tool entities.ToolReadOnly,
		opts ...IndexToolOption,
	) ([EmbeddingSize]float32, error)

	// BuildToolEmbedding generates query embedding from conversation context. Used to find
	// semantically relevant tools matching user's intent.
	//
	// See next test suites to find how it works:
	//
	//  - [TestToolEmbeddingSearch] — semantic search with various conversation
	//     contexts
	//
	// Throws:
	//
	//  - [ErrHardQuotaExhausted] if hard quota is exhausted. See
	//    documentation for [Port] to get more about quotas and
	//    rate limiting.
	//  - [PreflightFailedError] if preflight check fails. Note, that this error
	//    interprets as a business logic error, and if preflight failed due to
	//    infrastructure issues, callback is responsible to handle observability
	//    features.
	BuildToolEmbedding(
		ctx context.Context,
		msgs []messages.Message,
		opts ...BuildToolEmbeddingOption,
	) ([EmbeddingSize]float32, error)
}

func defaultIndexToolParams(required indexToolRequiredParams) indexToolParams {
	return indexToolParams{
		indexToolRequiredParams: required,
		preflight:               noopPreflight,
	}
}

func (p *indexToolParams) validate() error {
	return nil
}

func defaultBuildToolEmbeddingParams(
	required buildToolEmbeddingRequiredParams,
) buildToolEmbeddingParams {
	return buildToolEmbeddingParams{
		buildToolEmbeddingRequiredParams: required,
		preflight:                        noopPreflight,
	}
}

func (p *buildToolEmbeddingParams) validate() error {
	return nil
}

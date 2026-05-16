package embedding

import (
	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

// WithPreflightCheck sets the preflight check function for the request.
//
// Applies to:
//
//   - [Port.IndexTool]
//   - [Port.BuildToolEmbedding]
func WithPreflightCheck(check PreflightFunc) *preflightOption {
	if check == nil {
		check = noopPreflight
	}

	return &preflightOption{preflight: check}
}

type preflightOption struct{ preflight PreflightFunc }

func (o preflightOption) applyIndexTool(p *indexToolParams) { p.preflight = o.preflight }
func (o preflightOption) applyBuildToolEmbedding(p *buildToolEmbeddingParams) {
	p.preflight = o.preflight
}

type (
	IndexToolOption          interface{ applyIndexTool(p *indexToolParams) }
	BuildToolEmbeddingOption interface {
		applyBuildToolEmbedding(p *buildToolEmbeddingParams)
	}

	indexToolFunc          func(*indexToolParams)
	buildToolEmbeddingFunc func(*buildToolEmbeddingParams)
)

var (
	_ IndexToolOption          = indexToolFunc(nil)
	_ BuildToolEmbeddingOption = buildToolEmbeddingFunc(nil)
)

func (f indexToolFunc) applyIndexTool(p *indexToolParams) { f(p) }
func (f buildToolEmbeddingFunc) applyBuildToolEmbedding(p *buildToolEmbeddingParams) {
	f(p)
}

// ========================================================================== //
//                              [Port.IndexTool]                              //
// ========================================================================== //

type indexToolRequiredParams struct {
	tool entities.ToolReadOnly
}

type indexToolParams struct {
	indexToolRequiredParams
	preflight PreflightFunc
}

func IndexToolParams(
	tool entities.ToolReadOnly, opts ...IndexToolOption,
) (indexToolParams, error) {
	params := defaultIndexToolParams(indexToolRequiredParams{
		tool: tool,
	})
	for _, opt := range opts {
		opt.applyIndexTool(&params)
	}

	if err := params.validate(); err != nil {
		return indexToolParams{}, err
	}

	return params, nil
}

func (p *indexToolParams) Tool() entities.ToolReadOnly   { return p.tool }
func (p *indexToolParams) PreflightCheck() PreflightFunc { return p.preflight }

// ========================================================================== //
//                          [Port.BuildToolEmbedding]                         //
// ========================================================================== //

type buildToolEmbeddingRequiredParams struct {
	msgs []messages.Message
}

type buildToolEmbeddingParams struct {
	preflight PreflightFunc
	buildToolEmbeddingRequiredParams
}

func BuildToolEmbeddingParams(
	msgs []messages.Message, opts ...BuildToolEmbeddingOption,
) (buildToolEmbeddingParams, error) {
	params := defaultBuildToolEmbeddingParams(buildToolEmbeddingRequiredParams{
		msgs: msgs,
	})
	for _, opt := range opts {
		opt.applyBuildToolEmbedding(&params)
	}

	if err := params.validate(); err != nil {
		return buildToolEmbeddingParams{}, err
	}

	return params, nil
}

func (p *buildToolEmbeddingParams) Messages() []messages.Message  { return p.msgs }
func (p *buildToolEmbeddingParams) PreflightCheck() PreflightFunc { return p.preflight }

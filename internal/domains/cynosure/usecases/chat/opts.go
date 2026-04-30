package chat

import (
	"github.com/quenbyako/core"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/chatmodel"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/ratelimiter"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/toolclient"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
)

func WithObservability(obs core.Metrics) NewOption {
	return newFunc(func(p *newParams) { p.obs = obs })
}

func WithChatLimit(limit uint) NewOption {
	return newFunc(func(p *newParams) { p.chatLimit = limit })
}

func WithToolChoice(toolChoice tools.ToolChoice) GenerateResponseOption {
	return generateResponseFunc(func(params *generateResponseParams) {
		params.toolChoice = toolChoice
	})
}

// ========================================================================== //
//                                [types]                                     //
// ========================================================================== //

type (
	NewOption              interface{ applyNew(p *newParams) }
	GenerateResponseOption interface {
		applyGenerateResponse(p *generateResponseParams)
	}

	newFunc              func(*newParams)
	generateResponseFunc func(*generateResponseParams)
)

var (
	_ NewOption              = newFunc(nil)
	_ GenerateResponseOption = generateResponseFunc(nil)
)

func (f newFunc) applyNew(p *newParams)                                        { f(p) }
func (f generateResponseFunc) applyGenerateResponse(p *generateResponseParams) { f(p) }

// ========================================================================== //
//                                  [New]                                     //
// ========================================================================== //

type newRequiredParams struct {
	storage     ports.ThreadStorage
	model       chatmodel.Port
	tool        toolclient.Port
	indexer     ports.ToolSemanticIndex
	toolStorage ports.ToolStorage
	server      ports.ServerStorage
	account     ports.AccountStorage
	agents      ports.AgentStorage
	limiter     ratelimiter.Port
}

type newParams struct {
	newRequiredParams
	obs       core.Metrics
	chatLimit uint
}

func buildNewParams(
	storage ports.ThreadStorage,
	model chatmodel.Port,
	tool toolclient.Port,
	indexer ports.ToolSemanticIndex,
	toolStorage ports.ToolStorage,
	server ports.ServerStorage,
	account ports.AccountStorage,
	agents ports.AgentStorage,
	limiter ratelimiter.Port,
	opts ...NewOption,
) (newParams, error) {
	params := defaultNewParams(newRequiredParams{
		storage:     storage,
		model:       model,
		tool:        tool,
		indexer:     indexer,
		toolStorage: toolStorage,
		server:      server,
		account:     account,
		agents:      agents,
		limiter:     limiter,
	})
	for _, opt := range opts {
		opt.applyNew(&params)
	}

	if err := params.validate(); err != nil {
		return newParams{}, err
	}

	return params, nil
}

// ========================================================================== //
//                         [Usecase.GenerateResponse]                         //
// ========================================================================== //

type generateResponseRequiredParams struct {
	msg      messages.MessageUser
	threadID ids.ThreadID
}

type generateResponseParams struct {
	generateResponseRequiredParams
	toolChoice tools.ToolChoice
	model      ids.AgentID
}

func buildGenerateResponseParams(
	threadID ids.ThreadID,
	msg messages.MessageUser,
	opts ...GenerateResponseOption,
) (generateResponseParams, error) {
	params := defaultGenerateResponseParams(generateResponseRequiredParams{
		threadID: threadID,
		msg:      msg,
	})

	for _, opt := range opts {
		opt.applyGenerateResponse(&params)
	}

	if err := params.validate(); err != nil {
		return generateResponseParams{}, err
	}

	return params, nil
}

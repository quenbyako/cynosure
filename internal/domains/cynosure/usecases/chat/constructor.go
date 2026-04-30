package chat

import (
	"github.com/quenbyako/core"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/chatmodel"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/ratelimiter"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/toolclient"
)

const (
	defaultAgentLoopTurns = 10
	defaultChatLimit      = 20
)

type Usecase struct {
	obs              observable
	storage          ports.ThreadStorage
	model            chatmodel.Port
	tools            toolclient.Port
	indexer          ports.ToolSemanticIndex
	toolStorage      ports.ToolStorage
	servers          ports.ServerStorage
	accounts         ports.AccountStorage
	agents           ports.AgentStorage
	limiter          ratelimiter.Port
	agentLoopTurns   uint8
	defaultChatLimit uint
}

func defaultNewParams(required newRequiredParams) newParams {
	return newParams{
		newRequiredParams: required,
		obs:               core.NoopMetrics(),
		chatLimit:         defaultChatLimit,
	}
}

func (s *newParams) validate() error {
	switch {
	case s.storage == nil:
		return errInternalValidation("storage repository is required")
	case s.model == nil:
		return errInternalValidation("chat model is required")
	case s.tool == nil:
		return errInternalValidation("tool manager is required")
	case s.indexer == nil:
		return errInternalValidation("indexer is required")
	case s.toolStorage == nil:
		return errInternalValidation("tool storage is required")
	case s.server == nil:
		return errInternalValidation("server storage is required")
	case s.account == nil:
		return errInternalValidation("account storage is required")
	case s.agents == nil:
		return errInternalValidation("model settings storage is required")
	case s.limiter == nil:
		return errInternalValidation("rate limiter is required")
	default:
		return nil
	}
}

// New creates a new usecase instance.
//
// TODO: find a way, how to reduce amount of ports in usecases.
//
//nolint:funlen // it's impossible for now to reduce size, cause there are too many required ports
func New(
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
) (*Usecase, error) {
	params, err := buildNewParams(
		storage, model, tool, indexer, toolStorage, server, account, agents, limiter, opts...,
	)
	if err != nil {
		return nil, err
	}

	obs, err := newObservable(ports.StackFromCore(params.obs, pkgName))
	if err != nil {
		return nil, err
	}

	return &Usecase{
		storage:          storage,
		model:            model,
		tools:            tool,
		indexer:          indexer,
		toolStorage:      toolStorage,
		servers:          server,
		accounts:         account,
		agents:           agents,
		limiter:          limiter,
		agentLoopTurns:   defaultAgentLoopTurns,
		defaultChatLimit: params.chatLimit,
		obs:              obs,
	}, nil
}

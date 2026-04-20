package toolclient

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
)

type PortWrapped interface {
	Port

	_PortWrapped()
}

type toolClientWrapped struct {
	w Port
	t *observable
}

func (t *toolClientWrapped) _PortWrapped() {}

func Wrap(client Port, observable ports.ObserveStack) PortWrapped {
	if observable == nil {
		observable = ports.NoOpObserveStack()
	}

	t := toolClientWrapped{
		w: client,
		t: newObservable(observable),
	}

	return &t
}

func (t *toolClientWrapped) DiscoverTools(
	ctx context.Context,
	serverAddr *url.URL,
	account ids.AccountID,
	accountSlug, accountDesc string,
	opts ...DiscoverToolsOption,
) ([]tools.RawTool, error) {
	params := DiscoverToolsParams(opts...)

	hasToken := params.Token() != nil

	ctx, span := t.t.discoverTools(ctx, account.ID().String(), serverAddr.String(), hasToken)
	defer span.end()

	resolved := resolvedDiscoverToolsParams(params)

	res, err := t.w.DiscoverTools(ctx, serverAddr, account, accountSlug, accountDesc, resolved)
	span.recordError(err)

	//nolint:wrapcheck // should not wrap adapter errors
	return res, err
}

//nolint:ireturn // returns interface for polimorphism
func (t *toolClientWrapped) ExecuteTool(
	ctx context.Context,
	tool entities.ToolReadOnly,
	args map[string]json.RawMessage,
	toolCallID string,
) (messages.MessageTool, error) {
	ctx, span := t.t.executeTool(ctx, tool.Name(), args, toolCallID)
	defer span.end()

	res, err := t.w.ExecuteTool(ctx, tool, args, toolCallID)
	span.recordError(err)

	if res != nil {
		span.recordResponse(res.Content())
	}

	//nolint:wrapcheck // should not wrap adapter errors
	return res, err
}

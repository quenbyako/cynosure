package identitymanager

import (
	"context"

	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

type PortWrapped interface {
	Port

	_PortWrapped()
}

type portWrapped struct {
	w Port

	t *observable
}

var _ PortWrapped = (*portWrapped)(nil)

func (i *portWrapped) _PortWrapped() {}

//nolint:ireturn // standard port pattern: hiding implementation details
func Wrap(client Port, observable ports.ObserveStack) PortWrapped {
	if observable == nil {
		observable = ports.NoOpObserveStack()
	}

	t := portWrapped{
		w: client,
		t: newObservable(observable),
	}

	return &t
}

func (i *portWrapped) HasUser(ctx context.Context, id ids.UserID) (bool, error) {
	ctx, span := i.t.hasUser(ctx, id.ID().String())
	defer span.end()

	res, err := i.w.HasUser(ctx, id)
	span.recordError(err)

	return res, err
}

func (i *portWrapped) LookupUser(ctx context.Context, telegramID string) (ids.UserID, error) {
	ctx, span := i.t.lookupUser(ctx, telegramID)
	defer span.end()

	res, err := i.w.LookupUser(ctx, telegramID)
	span.recordError(err)

	return res, err
}

func (i *portWrapped) CreateUser(
	ctx context.Context, telegramID, nickname, firstName, lastName string,
) (ids.UserID, error) {
	ctx, span := i.t.createUser(ctx, telegramID, nickname, firstName, lastName)
	defer span.end()

	res, err := i.w.CreateUser(ctx, telegramID, nickname, firstName, lastName)
	span.recordError(err)

	//nolint:wrapcheck // should not wrap errors from adapters
	return res, err
}

func (i *portWrapped) IssueToken(ctx context.Context, id ids.UserID) (*oauth2.Token, error) {
	ctx, span := i.t.issueToken(ctx, id.ID().String())
	defer span.end()

	res, err := i.w.IssueToken(ctx, id)
	span.recordError(err)

	//nolint:wrapcheck // should not wrap errors from adapters
	return res, err
}

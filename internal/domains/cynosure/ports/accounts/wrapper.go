package accounts

import (
	"context"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

type PortWrapped interface {
	PortReadWrapped
	PortWriteWrapped
}

type portWrapped struct {
	portReadWrapped
	portWriteWrapped
}

var _ PortWrapped = (*portWrapped)(nil)

func (*portWrapped) _PortWrapped() {}

func Wrap(client Port, observable ports.ObserveStack) PortWrapped {
	if observable == nil {
		observable = ports.NoOpObserveStack()
	}

	obs := newObservable(observable)

	t := portWrapped{
		portReadWrapped: portReadWrapped{
			w: client,
			t: obs,
		},
		portWriteWrapped: portWriteWrapped{
			w: client,
			t: obs,
		},
	}

	return &t
}

type PortReadWrapped interface {
	PortRead
	_PortWrapped()
}

type portReadWrapped struct {
	w PortRead
	t *observable
}

var _ PortReadWrapped = (*portReadWrapped)(nil)

func (i *portReadWrapped) _PortWrapped() {}

func WrapRead(client PortRead, observable ports.ObserveStack) PortReadWrapped {
	if observable == nil {
		observable = ports.NoOpObserveStack()
	}

	t := portReadWrapped{
		w: client,
		t: newObservable(observable),
	}

	return &t
}

func (i *portReadWrapped) GetAccount(ctx context.Context, account ids.AccountID, opts ...GetAccountOption) (*entities.Account, error) {
	ctx, span := i.t.getAccount(ctx, account.String())
	defer span.end()

	res, err := i.w.GetAccount(ctx, account, opts...)
	span.recordError(err)

	//nolint:wrapcheck // should not wrap errors from adapters
	return res, err
}

func (i *portReadWrapped) GetAccountsBatch(ctx context.Context, accounts []ids.AccountID) ([]*entities.Account, error) {
	stringed := make([]string, len(accounts))
	for i, account := range accounts {
		stringed[i] = account.String()
	}

	ctx, span := i.t.getAccountsBatch(ctx, stringed)
	defer span.end()

	res, err := i.w.GetAccountsBatch(ctx, accounts)
	span.recordError(err)

	//nolint:wrapcheck // should not wrap errors from adapters
	return res, err
}

func (i *portReadWrapped) ListAccounts(ctx context.Context, user ids.UserID) ([]ids.AccountID, error) {
	ctx, span := i.t.listAccounts(ctx, user.ID().String())
	defer span.end()

	res, err := i.w.ListAccounts(ctx, user)
	span.recordError(err)

	//nolint:wrapcheck // should not wrap errors from adapters
	return res, err
}

type PortWriteWrapped interface {
	PortWrite
	_PortWrapped()
}

type portWriteWrapped struct {
	w PortWrite
	t *observable
}

var _ PortWriteWrapped = (*portWriteWrapped)(nil)

func (i *portWriteWrapped) _PortWrapped() {}

func (i *portWriteWrapped) DeleteAccount(ctx context.Context, account ids.AccountID) error {
	ctx, span := i.t.deleteAccount(ctx, account.String())
	defer span.end()

	err := i.w.DeleteAccount(ctx, account)
	span.recordError(err)

	//nolint:wrapcheck // should not wrap errors from adapters
	return err
}

func (i *portWriteWrapped) ReactivateAccount(ctx context.Context, account ids.AccountID) error {
	ctx, span := i.t.reactivateAccount(ctx, account.String())
	defer span.end()

	err := i.w.ReactivateAccount(ctx, account)
	span.recordError(err)

	//nolint:wrapcheck // should not wrap errors from adapters
	return err
}

func (i *portWriteWrapped) SaveAccount(ctx context.Context, info entities.AccountReadOnly) error {
	ctx, span := i.t.saveAccount(ctx, info.ID().String())
	defer span.end()

	err := i.w.SaveAccount(ctx, info)
	span.recordError(err)

	//nolint:wrapcheck // should not wrap errors from adapters
	return err
}

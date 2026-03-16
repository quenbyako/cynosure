package chatmodel

import (
	"context"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
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

//nolint:ireturn // it's a factory function
func Wrap(client Port, observable ports.ObserveStack) PortWrapped {
	if observable == nil {
		// TODO: FIX
		panic("required observable stack")
	}

	t := portWrapped{
		w: client,
		t: newObservable(observable),
	}

	return &t
}

func (i *portWrapped) Stream(
	ctx context.Context,
	input []messages.Message,
	settings entities.AgentReadOnly,
	opts ...StreamOption,
) (StreamIter, error) {
	params := StreamParams(opts...)

	ctx, span := i.t.stream(ctx, input, settings)

	res, err := i.w.Stream(ctx, input, settings, resolvedStreamParams(params))
	if err != nil {
		span.recordError(err)
		span.end()
		//nolint:wrapcheck // should never wrap error
		return nil, err
	}

	return func(yield func(messages.Message, error) bool) {
		defer span.end()

		res(func(msg messages.Message, err error) bool {
			if err != nil {
				span.recordError(err)
			} else {
				span.addOutputMessage(msg)
			}

			return yield(msg, err)
		})
	}, nil
}

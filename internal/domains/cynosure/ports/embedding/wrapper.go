package embedding

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

func (i *portWrapped) IndexTool(
	ctx context.Context,
	tool entities.ToolReadOnly,
	opts ...IndexToolOption,
) ([EmbeddingSize]float32, error) {
	ctx, span := i.t.indexTool(ctx)

	res, err := i.w.IndexTool(ctx, tool, opts...)
	if err != nil {
		span.recordError(err)
		span.end()
		//nolint:wrapcheck // should never wrap error
		return [EmbeddingSize]float32{}, err
	}

	return res, nil
}

func (i *portWrapped) BuildToolEmbedding(
	ctx context.Context,
	msgs []messages.Message,
	opts ...BuildToolEmbeddingOption,
) ([EmbeddingSize]float32, error) {
	ctx, span := i.t.buildToolEmbedding(ctx)

	res, err := i.w.BuildToolEmbedding(ctx, msgs, opts...)
	if err != nil {
		span.recordError(err)
		span.end()
		//nolint:wrapcheck // should never wrap error
		return [EmbeddingSize]float32{}, err
	}

	return res, nil
}

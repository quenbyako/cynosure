package telegram

import (
	"context"
	"sync/atomic"

	"github.com/alitto/pond/v2"
)

type requestPool[T any] struct {
	handler  func(context.Context, T)
	p        pond.Pool
	running  atomic.Bool
	finished atomic.Bool
}

func newRequestPool[T any](maxWorkers int, handler func(context.Context, T)) *requestPool[T] {
	return &requestPool[T]{
		running:  atomic.Bool{},
		finished: atomic.Bool{},
		handler:  handler,
		p:        pond.NewPool(maxWorkers),
	}
}

func (p *requestPool[T]) run(ctx context.Context) error {
	if !p.running.CompareAndSwap(false, true) {
		//nolint:forbidigo // panic helps here to catch bugs related to lifecycle
		panic("already running")
	}

	// waiting for shut down
	<-ctx.Done()

	p.finished.Store(true)
	p.p.StopAndWait()

	return nil
}

// returns true, if request was submitted, false otherwise
func (p *requestPool[T]) Submit(ctx contextValues, args T) bool {
	if !p.running.Load() || p.finished.Load() {
		return false
	}

	if ctx == nil {
		ctx = context.Background()
	}

	mergedCtx := mergedContext{Context: p.p.Context(), values: ctx}

	p.p.Submit(func() { p.handler(&mergedCtx, args) })

	return true
}

type contextValues interface{ Value(key any) any }

var _ contextValues = context.Context(nil)

type mergedContext struct {
	//nolint:containedctx // that's extension for context mechanism.
	context.Context
	values contextValues
}

var _ context.Context = (*mergedContext)(nil)

//nolint:ireturn // context.Value returns any
func (m *mergedContext) Value(key any) any {
	if value := m.values.Value(key); value != nil {
		return value
	}

	return m.Context.Value(key)
}

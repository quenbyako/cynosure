// Package taskpool provides a generic, lifecycle-aware task pool.
// It wraps github.com/alitto/pond/v2 to provide a type-safe handler-based
// approach for processing asynchronous tasks.
//
//nolint:ireturn // This bug happened again, [context.Context.Value] was false-positively flagged.
package taskpool

import (
	"context"
	"sync"

	"github.com/alitto/pond/v2"
)

// TaskPool represents a pool of workers that process tasks of type T
// using a predefined handler function.
type TaskPool[T any] struct {
	handler func(context.Context, T)
	pool    pond.Pool
	poolMu  sync.Mutex

	maxWorkers int
}

// New creates a new TaskPool with the given maxWorkers and task handler.
func New[T any](maxWorkers int, handler func(context.Context, T)) *TaskPool[T] {
	return &TaskPool[T]{
		handler:    handler,
		maxWorkers: maxWorkers,
		pool:       nil,
		poolMu:     sync.Mutex{},
	}
}

// Run starts the pool lifecycle and blocks until the context is canceled.
// When the context is canceled, it waits for all submitted tasks to finish.
func (p *TaskPool[T]) Run(ctx context.Context) error {
	p.poolMu.Lock()
	if p.pool != nil {
		//nolint:forbidigo // system-wide failure, absolutely unsafe to ignore
		panic("taskpool: already running")
	}

	// using WithoutCancel to prevent pool from being stopped when the main
	// context is canceled. main context works below. Here we are just saving
	// values that might be necessary for tasks.
	p.pool = pond.NewPool(p.maxWorkers, pond.WithContext(context.WithoutCancel(ctx)))
	p.poolMu.Unlock()

	// Wait for shutdown signal
	<-ctx.Done()
	p.pool.StopAndWait()

	return nil
}

func (p *TaskPool[T]) Running() bool {
	p.poolMu.Lock()
	defer p.poolMu.Unlock()

	return p.pool != nil && !p.pool.Stopped()
}

// Submit submits a task for asynchronous processing.
// It returns true if the task was submitted, and false if the pool is not running
// or has already finished.
//
// The context values from the provided ctx are merged with the pool's
// internal context to ensure that values like Trace ID or User ID are
// propagated to the handler.
func (p *TaskPool[T]) Submit(ctx ContextValues, task T) bool {
	if p.pool == nil {
		return false
	}

	// Merge context values from the caller with the pool's lifecycle context
	mergedCtx := &mergedContext{
		Context: p.pool.Context(),

		// see [mergedContext] for explanation of nil safety
		values: ctx,
	}

	p.pool.Submit(func() {
		p.handler(mergedCtx, task)
	})

	// NOTE: that's too tricky and unsafe way to check if the pool is running,
	// but that's the only way with pond. It's a really great idea to contribude
	// to lib to add in Task interface Submitted() method or sorta.
	return !p.pool.Stopped()
}

// ContextValues is an interface that allows extracting values from a context.
type ContextValues interface {
	Value(key any) any
}

// mergedContext wraps a parent context but prioritizes values from another
// context provider.
type mergedContext struct {
	//nolint:containedctx // false positive
	context.Context

	// Important: values MAY be nil, and this is completely safe for execution
	// context: [mergedContext.Context] is responsible for execution lifecycle,
	// while values is related only to values propagation and nothing more.
	values ContextValues
}

var _ context.Context = (*mergedContext)(nil)

// Value returns the value associated with this context for key, or nil if no
// value is associated with key. It first checks the values provider, then falls
// back to the parent context.
func (m *mergedContext) Value(key any) any {
	if m.values != nil {
		if value := m.values.Value(key); value != nil {
			return value
		}
	}

	return m.Context.Value(key)
}

// Package taskpool provides a generic, lifecycle-aware task pool.
// It wraps github.com/alitto/pond/v2 to provide a type-safe handler-based
// approach for processing asynchronous tasks.
package taskpool

import (
	"context"
	"sync/atomic"

	"github.com/alitto/pond/v2"
)

// TaskPool represents a pool of workers that process tasks of type T
// using a predefined handler function.
type TaskPool[T any] struct {
	handler  func(context.Context, T)
	pool     pond.Pool
	running  atomic.Bool
	finished atomic.Bool
}

// New creates a new TaskPool with the given maxWorkers and task handler.
func New[T any](maxWorkers int, handler func(context.Context, T)) *TaskPool[T] {
	return &TaskPool[T]{
		handler:  handler,
		pool:     pond.NewPool(maxWorkers),
		running:  atomic.Bool{},
		finished: atomic.Bool{},
	}
}

// Run starts the pool lifecycle and blocks until the context is canceled.
// When the context is canceled, it waits for all submitted tasks to finish.
func (p *TaskPool[T]) Run(ctx context.Context) error {
	if !p.running.CompareAndSwap(false, true) {
		//nolint:forbidigo // system-wide failure, absolutely unsafe to ignore
		panic("taskpool: already running")
	}

	// Wait for shutdown signal
	<-ctx.Done()

	p.finished.Store(true)
	p.pool.StopAndWait()

	return nil
}

// Running returns true if the pool is running and has not yet finished.
func (p *TaskPool[T]) Running() bool { return p.running.Load() && !p.finished.Load() }

// Submit submits a task for asynchronous processing.
// It returns true if the task was submitted, and false if the pool is not running
// or has already finished.
//
// The context values from the provided ctx are merged with the pool's
// internal context to ensure that values like Trace ID or User ID are
// propagated to the handler.
func (p *TaskPool[T]) Submit(ctx ContextValues, task T) bool {
	if !p.Running() {
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

	return true
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
//
//nolint:ireturn // implementation of [context.Context]
func (m *mergedContext) Value(key any) any {
	if m.values != nil {
		if value := m.values.Value(key); value != nil {
			return value
		}
	}

	return m.Context.Value(key)
}

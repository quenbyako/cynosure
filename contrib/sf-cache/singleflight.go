// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package cache provides a duplicate function call suppression mechanism,
// similar to golang.org/x/sync/singleflight, but with generic support and context-awareness.
package cache

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
)

// panicError is an arbitrary value recovered from a panic with the stack trace.
type panicError struct {
	value any
	stack []byte
}

func (p *panicError) Error() string {
	return fmt.Sprintf("%v\n\n%s", p.value, p.stack)
}

func (p *panicError) Unwrap() error {
	if err, ok := p.value.(error); ok {
		return err
	}

	return nil
}

func newPanicError(val any) error {
	stack := debug.Stack()

	// The first line of the stack trace is of the form "goroutine N [status]:"
	// but by the time the panic reaches Do, the goroutine may no longer exist.
	// Trim out the first line.
	if line := bytes.IndexByte(stack, '\n'); line >= 0 {
		stack = stack[line+1:]
	}

	return &panicError{value: val, stack: stack}
}

// call represents an in-flight or completed singleflight.Do call.
type call[T any] struct {
	err  error
	val  T
	wg   sync.WaitGroup
	dups atomic.Int32
}

// group represents a class of work and forms a namespace in which units of work
// can be executed with duplicate suppression.
type group[T any] struct {
	current *call[T]
	mu      sync.Mutex // protects current
}

// Do executes and returns the results of the given function, making sure that
// only one execution is in-flight for a given key at a time.
func (g *group[T]) Do(
	ctx context.Context,
	executeConstructor func(ctx context.Context) (T, error),
) (T, error, bool) {
	g.mu.Lock()

	if activeCall := g.current; activeCall != nil {
		g.mu.Unlock()

		activeCall.dups.Add(1)
		activeCall.wg.Wait()

		g.handlePanicOrGoexit(activeCall.err)

		return activeCall.val, activeCall.err, true
	}

	newCall := &call[T]{
		err:  nil,
		val:  *new(T),
		wg:   sync.WaitGroup{},
		dups: atomic.Int32{},
	}

	newCall.wg.Add(1)
	g.current = newCall
	g.mu.Unlock()

	g.doCall(ctx, newCall, executeConstructor)

	return newCall.val, newCall.err, newCall.dups.Load() > 0
}

// Get returns the current result if any.
func (g *group[T]) Get() (v T, err error, duplicate bool) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.current != nil {
		return g.current.val, g.current.err, true
	}

	var zero T

	return zero, nil, false
}

// Forget tells the singleflight to forget about a key.
func (g *group[T]) Forget() {
	g.mu.Lock()
	g.current = nil
	g.mu.Unlock()
}

func (g *group[T]) doCall(
	ctx context.Context,
	activeCall *call[T],
	executeConstructor func(ctx context.Context) (T, error),
) {
	normalReturn := false
	recovered := false

	defer func() {
		if !normalReturn && !recovered {
			activeCall.err = errGoexit
		}

		activeCall.wg.Done()
		g.handlePanicOrGoexit(activeCall.err)
	}()

	normalReturn = g.run(ctx, activeCall, executeConstructor)

	if !normalReturn {
		recovered = true
	}
}

func (g *group[T]) run(
	ctx context.Context,
	activeCall *call[T],
	executeConstructor func(ctx context.Context) (T, error),
) (normalReturn bool) {
	defer func() {
		if !normalReturn {
			if caught := recover(); caught != nil {
				activeCall.err = newPanicError(caught)
			}
		}
	}()

	activeCall.val, activeCall.err = executeConstructor(ctx)

	return true
}

// handlePanicOrGoexit re-throws a panic or Goexit if the error indicates one occurred.
func (g *group[T]) handlePanicOrGoexit(err error) {
	var pErr *panicError
	if errors.As(err, &pErr) {
		//nolint:forbidigo // Re-panicking is intended behavior for singleflight.
		panic(pErr)
	}

	if errors.Is(err, errGoexit) {
		runtime.Goexit()
	}
}

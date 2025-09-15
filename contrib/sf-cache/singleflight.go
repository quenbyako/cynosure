// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package singleflight provides a duplicate function call suppression
// mechanism.
package cache

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"runtime"
	"runtime/debug"
	"sync"
)

// errGoexit indicates the runtime.Goexit was called in
// the user given function.
var errGoexit = errors.New("runtime.Goexit was called")

// A panicError is an arbitrary value recovered from a panic
// with the stack trace during the execution of given function.
type panicError struct {
	value any
	stack []byte
}

// Error implements error interface.
func (p *panicError) Error() string {
	return fmt.Sprintf("%v\n\n%s", p.value, p.stack)
}

func (p *panicError) Unwrap() error {
	err, ok := p.value.(error)
	if !ok {
		return nil
	}

	return err
}

func newPanicError(v any) error {
	stack := debug.Stack()

	// The first line of the stack trace is of the form "goroutine N [status]:"
	// but by the time the panic reaches Do the goroutine may no longer exist
	// and its status will have changed. Trim out the misleading line.
	if line := bytes.IndexByte(stack[:], '\n'); line >= 0 {
		stack = stack[line+1:]
	}
	return &panicError{value: v, stack: stack}
}

// call is an in-flight or completed singleflight.Do call
type call[T any] struct {
	wg sync.WaitGroup

	// These fields are written once before the WaitGroup is done
	// and are only read after the WaitGroup is done.
	val T
	err error

	// These fields are read and written with the singleflight
	// mutex held before the WaitGroup is done, and are read but
	// not written after the WaitGroup is done.
	dups int
}

// group represents a class of work and forms a namespace in
// which units of work can be executed with duplicate suppression.
type group[T any] struct {
	mu sync.Mutex // protects m
	c  *call[T]   // lazily initialized
}

func NewGroup() *group[any] {
	return &group[any]{
		mu: sync.Mutex{},
		c:  nil,
	}
}

// Do executes and returns the results of the given function, making
// sure that only one execution is in-flight for a given key at a
// time. If a duplicate comes in, the duplicate caller waits for the
// original to complete and receives the same results.
// The return value shared indicates whether v was given to multiple callers.
func (g *group[T]) Do(ctx context.Context, fn func(ctx context.Context) (T, error)) (v T, err error, shared bool) {
	g.mu.Lock()
	if g.c != nil {
		g.c.dups++
		g.mu.Unlock()
		g.c.wg.Wait()

		if e, ok := g.c.err.(*panicError); ok {
			panic(e)
		} else if g.c.err == errGoexit {
			runtime.Goexit()
		}
		return g.c.val, g.c.err, true
	}

	g.c = &call[T]{}
	g.c.wg.Add(1)
	g.mu.Unlock()

	g.doCall(ctx, fn)
	return g.c.val, g.c.err, g.c.dups > 0
}

func (g *group[T]) Get() (v T, err error, ok bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.c != nil {
		return g.c.val, g.c.err, true
	}

	return v, nil, false
}

// Forget tells the singleflight to forget about a key.  Future calls
// to Do for this key will call the function rather than waiting for
// an earlier call to complete.
func (g *group[T]) Forget() {
	g.mu.Lock()
	g.c = nil
	g.mu.Unlock()
}

// doCall handles the single call for a key.
func (g *group[T]) doCall(ctx context.Context, fn func(ctx context.Context) (T, error)) {
	normalReturn := false
	recovered := false

	// use double-defer to distinguish panic from runtime.Goexit,
	// more details see https://golang.org/cl/134395
	defer func() {
		// the given function invoked runtime.Goexit
		if !normalReturn && !recovered {
			g.c.err = errGoexit
		}

		g.mu.Lock()
		defer g.mu.Unlock()
		g.c.wg.Done()

		if e, ok := g.c.err.(*panicError); ok {
			panic(e)
		} else if g.c.err == errGoexit {
			// Already in the process of goexit, no need to call again
		}
	}()

	func() {
		defer func() {
			if !normalReturn {
				// Ideally, we would wait to take a stack trace until we've determined
				// whether this is a panic or a runtime.Goexit.
				//
				// Unfortunately, the only way we can distinguish the two is to see
				// whether the recover stopped the goroutine from terminating, and by
				// the time we know that, the part of the stack trace relevant to the
				// panic has been discarded.
				if r := recover(); r != nil {
					g.c.err = newPanicError(r)
				}
			}
		}()

		g.c.val, g.c.err = fn(ctx)
		normalReturn = true
	}()

	if !normalReturn {
		recovered = true
	}
}

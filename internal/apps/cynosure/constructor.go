package cynosure

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

type constructor[T any] interface {
	Build(ctx context.Context) (T, error)
	Initiated() bool
}

type noopConstructor[T any] struct{}

var _ constructor[any] = (*noopConstructor[any])(nil)

func (*noopConstructor[T]) Build(context.Context) (T, error) { return *new(T), nil }

func (*noopConstructor[T]) Initiated() bool { return false }

type initFunc[T any] func(context.Context) (T, error)

func construct[T any](fn func(context.Context) (T, error)) constructor[T] {
	return initFunc[T](onceFn(fn))
}

func (f initFunc[T]) Build(ctx context.Context) (T, error) { return f(ctx) }

func (initFunc[T]) Initiated() bool { return true }

func onceFn[T1, T2 any](f func(context.Context) (T1, T2)) func(context.Context) (T1, T2) {
	// Use a struct so that there's a single heap allocation.
	d := struct {
		f     func(context.Context) (T1, T2)
		once  sync.Once
		valid bool
		p     any
		r1    T1
		r2    T2
	}{
		f: f,
	}
	return func(ctx context.Context) (T1, T2) {
		d.once.Do(func() {
			defer func() {
				d.f = nil
				d.p = recover()
				if !d.valid {
					panic(d.p)
				}
			}()
			d.r1, d.r2 = d.f(ctx)
			d.valid = true
		})
		if !d.valid {
			panic(d.p)
		}
		return d.r1, d.r2
	}
}

func fallbackAdapter[T, P any](c constructor[T], selected bool, mapFn func(T) (P, error)) portChoice[P] {
	priority := priorityFallback
	if selected {
		priority = priorityPreferred
	}

	return adapter(c, priority, mapFn)
}

func configuredAdapter[T, P any](c constructor[T], selected bool, mapFn func(T) (P, error)) portChoice[P] {
	priority := priorityConfigured
	if selected {
		priority = priorityPreferred
	}

	return adapter(c, priority, mapFn)
}

func adapter[T, P any](c constructor[T], priority choicePriority, mapFn func(T) (P, error)) portChoice[P] {
	return portChoice[P]{
		build: func(ctx context.Context) (P, error) {
			t, err := c.Build(ctx)
			if err != nil {
				return *new(P), err
			}

			return mapFn(t)
		},
		initiated: c.Initiated(),
		priority:  priority,
	}
}

type portChoice[Port any] struct {
	build     func(context.Context) (Port, error)
	initiated bool
	priority  choicePriority
}

func selectPort[T any](ctx context.Context, choices ...portChoice[T]) (T, error) {
	var best []portChoice[T]
	var maxPrio choicePriority

	for _, c := range choices {
		if !c.initiated && c.priority != priorityPreferred {
			continue
		}

		if c.priority > maxPrio {
			maxPrio = c.priority
			best = []portChoice[T]{c}
		} else if c.priority == maxPrio && c.priority != priorityNone {
			best = append(best, c)
		}
	}

	if len(best) == 0 {
		return *new(T), errors.New("no suitable adapter found")
	}

	if len(best) > 1 {
		return *new(T), fmt.Errorf("ambiguous adapter configuration at priority level %v", maxPrio)
	}

	if !best[0].initiated {
		return *new(T), errors.New("preferred adapter is explicitly selected but not configured")
	}

	return best[0].build(ctx)
}

type choicePriority uint8

const (
	priorityNone choicePriority = iota
	priorityFallback
	priorityConfigured
	priorityPreferred
)

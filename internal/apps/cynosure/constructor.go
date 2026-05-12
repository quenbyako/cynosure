package cynosure

import (
	"context"
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

type onceData[T1, T2 any] struct {
	p     any
	r1    T1
	r2    T2
	f     func(context.Context) (T1, T2)
	once  sync.Once
	valid bool
}

func (d *onceData[T1, T2]) call(ctx context.Context) (r1 T1, r2 T2) {
	d.once.Do(func() {
		defer func() {
			d.f = nil
			d.p = recover()

			if !d.valid {
				panic(d.p) //nolint:forbidigo // onceFn re-panics if the initial call panicked.
			}
		}()

		d.r1, d.r2 = d.f(ctx)
		d.valid = true
	})

	if !d.valid {
		panic(d.p) //nolint:forbidigo // onceFn re-panics if the initial call panicked.
	}

	return d.r1, d.r2
}

func onceFn[T1, T2 any](initFn func(context.Context) (T1, T2)) func(context.Context) (T1, T2) {
	data := &onceData[T1, T2]{
		p:     nil,
		r1:    *new(T1),
		r2:    *new(T2),
		f:     initFn,
		once:  sync.Once{},
		valid: false,
	}

	return data.call
}

/*
func fallbackAdapter[T, P any](
	c constructor[T], selected bool, mapFn func(T) (P, error),
) portChoice[P] {
	priority := priorityFallback
	if selected {
		priority = priorityPreferred
	}

	return adapter(c, priority, mapFn)
}
*/

func configuredAdapter[T, P any](
	instance constructor[T], selected bool, mapFn func(T) (P, error),
) portChoice[P] {
	priority := priorityConfigured
	if selected {
		priority = priorityPreferred
	}

	return adapter(instance, priority, mapFn)
}

func adapter[T, P any](
	instance constructor[T], priority choicePriority, mapFn func(T) (P, error),
) portChoice[P] {
	return portChoice[P]{
		build: func(ctx context.Context) (P, error) {
			t, err := instance.Build(ctx)
			if err != nil {
				return *new(P), fmt.Errorf("building adapter: %w", err)
			}

			return mapFn(t)
		},
		initiated: instance.Initiated(),
		priority:  priority,
	}
}

type portChoice[Port any] struct {
	build     func(context.Context) (Port, error)
	initiated bool
	priority  choicePriority
}

func selectPort[T any](ctx context.Context, choices ...portChoice[T]) (T, error) {
	best, maxPrio := filterBestChoices(choices)

	if len(best) == 0 {
		return *new(T), ErrNoSuitableAdapter
	}

	if len(best) > 1 {
		return *new(T), fmt.Errorf("%w at priority level %v", ErrAmbiguousAdapter, maxPrio)
	}

	if !best[0].initiated {
		return *new(T), ErrPreferredAdapterNotConfigured
	}

	return best[0].build(ctx)
}

func filterBestChoices[T any](choices []portChoice[T]) ([]portChoice[T], choicePriority) {
	var (
		best    []portChoice[T]
		maxPrio choicePriority
	)

	for _, choice := range choices {
		if !choice.initiated && choice.priority != priorityPreferred {
			continue
		}

		if choice.priority > maxPrio {
			maxPrio = choice.priority
			best = []portChoice[T]{choice}
		} else if choice.priority == maxPrio && choice.priority != priorityNone {
			best = append(best, choice)
		}
	}

	return best, maxPrio
}

type choicePriority uint8

const (
	priorityNone choicePriority = iota
	priorityFallback
	priorityConfigured
	priorityPreferred
)

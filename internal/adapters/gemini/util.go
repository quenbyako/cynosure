package gemini

import (
	"errors"
	"iter"
	"sync"
)

// SafeMap wraps an iterator with a mapper function.
func SafeMap[K1, V1, K2 any](
	seq iter.Seq2[K1, V1],
	mapper func(K1, V1) (K2, error),
) iter.Seq2[K2, error] {
	return func(yield func(K2, error) bool) {
		seq(func(k1 K1, v1 V1) bool {
			val, err := mapper(k1, v1)
			if err != nil {
				yield(val, err)
				return false
			}

			return yield(val, nil)
		})
	}
}

// IterExtract flattens an iterator of slices into an iterator of elements.
func IterExtract[K1 any](seq iter.Seq2[[]K1, error]) iter.Seq2[K1, error] {
	return func(yield func(K1, error) bool) {
		seq(func(parts []K1, err error) bool {
			if err != nil {
				yield(*new(K1), err)
				return false
			}

			for _, item := range parts {
				if !yield(item, nil) {
					return false
				}
			}

			return true
		})
	}
}

type IterCloser[V, T any] interface {
	Next() (V, bool)
	Close() (T, error)
}

type iterWrapper[V, T any] struct {
	wrapped    IterCloser[V, T]
	middleware func(V, bool) (V, bool)
}

func WrapIter[V, T any](
	wrapped IterCloser[V, T],
	middleware func(V, bool) (V, bool),
) IterCloser[V, T] {
	return &iterWrapper[V, T]{
		wrapped:    wrapped,
		middleware: middleware,
	}
}

func (i *iterWrapper[V, T]) Next() (V, bool) {
	return i.middleware(i.wrapped.Next())
}

func (i *iterWrapper[V, T]) Close() (T, error) {
	return i.wrapped.Close()
}

type iterCloser[K1, K2, T any] struct {
	mapper    func(K1) ([]K2, error)
	collector func(T, K1) T

	mu sync.Mutex

	stream      func() (K1, error, bool)
	streamClose func()

	cached   []K2
	finished bool
	state    T
	err      error
}

func NewIterCloser[K1, K2, T any](
	stream iter.Seq2[K1, error],
	mapper func(K1) ([]K2, error),
	collector func(T, K1) T,
) IterCloser[K2, T] {
	next, close := iter.Pull2(stream)

	return &iterCloser[K1, K2, T]{
		mapper:    mapper,
		collector: collector,

		mu: sync.Mutex{},

		stream:      next,
		streamClose: close,

		cached:   nil,
		finished: false,
		state:    *new(T),
		err:      nil,
	}
}

func (s *iterCloser[K1, K2, T]) Next() (next K2, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.finished || s.err != nil {
		return *new(K2), false
	}

	if len(s.cached) > 0 {
		next, s.cached = s.cached[0], s.cached[1:]

		return next, true
	}

	for {
		nextRaw, nextErr, ok := s.stream()
		if !ok {
			return *new(K2), false
		}

		if nextErr != nil {
			s.err = nextErr
			return *new(K2), false
		}

		// mapping

		s.state = s.collector(s.state, nextRaw)

		converted, err := s.mapper(nextRaw)
		if err != nil {
			s.err = err
			return *new(K2), false
		}

		switch len(converted) {
		case 0:
			continue
		case 1:
			return converted[0], true
		default:
			s.cached = converted[1:]
			return converted[0], true
		}

	}
}

func (s *iterCloser[K1, K2, T]) Close() (T, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.finished {
		return s.state, s.err
	}

	var lastErr error

	// draining the stream
	for {
		val, err, ok := s.stream()
		if !ok {
			break
		}

		if err != nil {
			lastErr = err
		}

		s.state = s.collector(s.state, val)
	}

	s.streamClose()

	if lastErr != nil {
		if s.err == nil {
			s.err = lastErr
		} else {
			s.err = errors.Join(s.err, lastErr)
		}
	}

	s.finished = true
	return s.state, s.err
}

package gemini

import (
	"iter"
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

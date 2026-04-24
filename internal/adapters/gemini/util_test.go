package gemini_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	. "github.com/quenbyako/cynosure/internal/adapters/gemini"
)

func TestIterCloser_Next(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		stream := func(yield func(int, error) bool) {
			yield(1, nil)
			yield(2, nil)
		}
		mapper := func(i int) ([]string, error) {
			return []string{string(rune('a' + i - 1))}, nil
		}
		collector := func(state []int, i int) []int { return append(state, i) }

		iterCloser := NewIterCloser(stream, mapper, collector)

		val, ok := iterCloser.Next()
		require.True(t, ok)
		require.Equal(t, "a", val)

		val, ok = iterCloser.Next()
		require.True(t, ok)
		require.Equal(t, "b", val)

		_, ok = iterCloser.Next()
		require.False(t, ok)

		state, err := iterCloser.Close()
		require.NoError(t, err)
		require.Equal(t, []int{1, 2}, state)
	})

	t.Run("flattening", func(t *testing.T) {
		stream := func(yield func(int, error) bool) {
			yield(1, nil)
		}
		mapper := func(i int) ([]string, error) {
			return []string{"a", "b"}, nil
		}
		collector := func(i, state int) int {
			return state + i
		}

		iterCloser := NewIterCloser(stream, mapper, collector)

		val, ok := iterCloser.Next()
		require.True(t, ok)
		require.Equal(t, "a", val)

		val, ok = iterCloser.Next()
		require.True(t, ok)
		require.Equal(t, "b", val)

		_, ok = iterCloser.Next()
		require.False(t, ok)

		state, err := iterCloser.Close()
		require.NoError(t, err)
		require.Equal(t, 1, state)
	})

	t.Run("empty response from mapper", func(t *testing.T) {
		stream := func(yield func(int, error) bool) {
			yield(1, nil)
			yield(2, nil)
		}
		mapper := func(i int) ([]string, error) {
			if i == 1 {
				return nil, nil
			}

			return []string{"b"}, nil
		}
		collector := func(i, state int) int { return state + i }

		iterCloser := NewIterCloser(stream, mapper, collector)

		val, ok := iterCloser.Next()
		require.True(t, ok)
		require.Equal(t, "b", val)

		_, ok = iterCloser.Next()
		require.False(t, ok)

		state, err := iterCloser.Close()
		require.NoError(t, err)
		require.Equal(t, 3, state)
	})

	t.Run("error in stream", func(t *testing.T) {
		expectedErr := errors.New("stream error")
		stream := func(yield func(int, error) bool) {
			yield(1, nil)
			yield(0, expectedErr)
		}
		mapper := func(i int) ([]string, error) { return []string{"val"}, nil }
		collector := func(i, state int) int { return state + i }

		iterCloser := NewIterCloser(stream, mapper, collector)

		iterCloser.Next()
		_, ok := iterCloser.Next()
		require.False(t, ok)

		_, err := iterCloser.Close()
		require.ErrorIs(t, err, expectedErr)
	})

	t.Run("error in mapper", func(t *testing.T) {
		expectedErr := errors.New("mapper error")
		stream := func(yield func(int, error) bool) {
			yield(1, nil)
		}
		mapper := func(i int) ([]string, error) { return nil, expectedErr }
		collector := func(i, state int) int { return state + i }

		ic := NewIterCloser(stream, mapper, collector)

		_, ok := ic.Next()
		require.False(t, ok)

		_, err := ic.Close()
		require.ErrorIs(t, err, expectedErr)
	})
}

func TestIterCloser_Close(t *testing.T) {
	t.Run("draining the stream", func(t *testing.T) {
		stream := func(yield func(int, error) bool) {
			yield(1, nil)
			yield(2, nil)
		}
		mapper := func(i int) ([]string, error) { return []string{"val"}, nil }
		collector := func(i, state int) int { return state + i }

		ic := NewIterCloser(stream, mapper, collector)

		// Close without calling next
		state, err := ic.Close()
		require.NoError(t, err)
		require.Equal(t, 3, state) // ensuring that all data was drained
	})
}

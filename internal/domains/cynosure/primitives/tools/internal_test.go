package tools_test

import (
	"testing"
)

func must[T any](tb testing.TB) func(T, error) T {
	tb.Helper()

	return func(val T, err error) T {
		tb.Helper()

		if err != nil {
			tb.Fatal(err)
		}

		return val
	}
}

package cache

import (
	"errors"
)

var (
	// ErrClosed is returned when an operation is attempted on a closed cache.
	ErrClosed = errors.New("closed")

	// ErrEvicted is returned internally when a cache entry is removed during processing.
	ErrEvicted = errors.New("entry was evicted from cache")

	// errGoexit indicates the runtime.Goexit was called in
	// the user given function.
	errGoexit = errors.New("runtime.Goexit was called")
)

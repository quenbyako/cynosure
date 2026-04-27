package cache_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	. "github.com/quenbyako/cynosure/contrib/sf-cache"
)

// TestCache_StressEviction verifies that resources are never leaked or double-destructed
// even when entries are evicted while their constructors are still running.
func TestCache_StressEviction(t *testing.T) {
	const (
		numGoroutines = 100
		numKeys       = 50
		maxSize       = 5
		duration      = 2 * time.Second
	)

	var (
		constructorCalls atomic.Int32
		destructorCalls  atomic.Int32
		successCalls     atomic.Int32

		mu              sync.Mutex
		activeResources = make(map[string]bool)
		leakedResources []string
	)

	constructor := func(ctx context.Context, key string) (string, error) {
		constructorCalls.Add(1)

		// Simulate work and give time for eviction to happen
		time.Sleep(10 * time.Millisecond)

		val := "val-" + key

		mu.Lock()
		activeResources[val] = true
		mu.Unlock()

		successCalls.Add(1)

		return val, nil
	}

	destructor := func(key, val string) {
		destructorCalls.Add(1)

		mu.Lock()
		if !activeResources[val] {
			leakedResources = append(leakedResources,
				fmt.Sprintf("key %s: double destruct or destruct never created val %s", key, val),
			)
		}

		delete(activeResources, val)
		mu.Unlock()
	}

	testCache := New(constructor, destructor, maxSize, 1*time.Hour)

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	var wg sync.WaitGroup
	for i := range numGoroutines {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				default:
					key := fmt.Sprintf("key-%d", id%numKeys)

					//nolint:errcheck // Testing eviction.
					testCache.Get(ctx, key)

					// Small jitter to vary request patterns
					time.Sleep(time.Duration(id%5) * time.Millisecond)
				}
			}
		}(i)
	}

	wg.Wait()

	// Close should trigger destruction of remaining entries
	require.NoError(t, testCache.Close())

	// Give a small grace period for any background cleanups if they existed
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	remaining := len(activeResources)
	leaks := len(leakedResources)
	mu.Unlock()

	t.Logf("Constructor calls: %d", constructorCalls.Load())
	t.Logf("Success calls: %d", successCalls.Load())
	t.Logf("Destructor calls: %d", destructorCalls.Load())
	t.Logf("Remaining resources: %d", remaining)

	require.Equal(t, 0, leaks, "Found invalid destructions: %v", leakedResources)
	require.Equal(t, int(successCalls.Load()), int(destructorCalls.Load()),
		"Mismatch between successful creations and destructions. Resources might be leaked!")
	require.Equal(t, 0, remaining, "Some resources were not cleaned up")
}

// TestCache_CloseWhileInFlight specifically targets the shutdown scenario
// where multiple requests are waiting for a value that is being evicted/closed.
func TestCache_CloseWhileInFlight(t *testing.T) {
	var (
		constructorStarted  atomic.Int32
		constructorFinished atomic.Int32
		destructorCalled    atomic.Int32
	)

	startCh := make(chan struct{})
	finishCh := make(chan struct{})

	constructor := func(ctx context.Context, key string) (string, error) {
		constructorStarted.Add(1)
		close(startCh)
		<-finishCh
		constructorFinished.Add(1)

		return "expensive-resource", nil
	}

	destructor := func(key, val string) {
		destructorCalled.Add(1)
	}

	testCache := New(constructor, destructor, 10, 1*time.Hour)

	// Start a request that blocks in constructor
	//nolint:errcheck // Testing eviction.
	go testCache.Get(context.Background(), "key1")

	// Wait for it to start
	<-startCh

	// Now close the cache while constructor is still running
	closeErr := make(chan error, 1)

	go func() {
		closeErr <- testCache.Close()
	}()

	// Give it a moment to enter Resize(0) and wait on group.Get()
	time.Sleep(50 * time.Millisecond)

	// Now allow the constructor to finish
	close(finishCh)

	require.NoError(t, <-closeErr)

	// Verify that destructor was called
	require.Equal(t, int32(1), destructorCalled.Load(),
		"Destructor should be called after constructor finishes",
	)
	require.Equal(t, int32(1), constructorFinished.Load())
}

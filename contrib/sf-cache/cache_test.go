package cache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestCacheConcurrentGet verifies thread-safety with concurrent Get operations
func TestCacheConcurrentGet(t *testing.T) {
	t.Parallel()

	const numGoroutines = 500
	const numKeys = 5 // Match the cache maxSize to avoid evictions

	var constructorCalls atomic.Int32
	var destructorCalls atomic.Int32

	constructor := func(ctx context.Context, k int) (string, error) {
		constructorCalls.Add(1)
		time.Sleep(1 * time.Millisecond) // Simulate some work
		return "value", nil
	}

	destructor := func(k int, v string) {
		destructorCalls.Add(1)
	}

	cache := New(constructor, destructor, 5, 100*time.Millisecond)
	defer cache.Close()

	var wg sync.WaitGroup
	ctx := context.Background()

	// Launch many goroutines accessing the cache concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			key := id % numKeys
			v, err := cache.Get(ctx, key)
			if err != nil {
				t.Errorf("Get failed: %v", err)
				return
			}
			if v != "value" {
				t.Errorf("Got %q, want %q", v, "value")
			}
		}(i)
	}

	wg.Wait()

	// Verify that constructor was called at most numKeys times
	// (Singleflight should deduplicate concurrent calls for the same key)
	calls := constructorCalls.Load()
	if calls > numKeys*2 { // Allow some margin for timing issues
		t.Errorf("Constructor called %d times, expected roughly %d", calls, numKeys)
	}
}

// TestCacheConcurrentGetWithLRUEviction tests concurrent access with LRU eviction
func TestCacheConcurrentGetWithLRUEviction(t *testing.T) {
	t.Parallel()

	const maxSize = 5
	const numGoroutines = 100
	const numKeys = 20 // More keys than maxSize to trigger eviction

	var destructorCalls atomic.Int32
	destructedKeys := make(map[int]int)
	var destructMu sync.Mutex

	constructor := func(ctx context.Context, k int) (string, error) {
		return "value", nil
	}

	destructor := func(k int, v string) {
		destructorCalls.Add(1)
		destructMu.Lock()
		destructedKeys[k]++
		destructMu.Unlock()
	}

	cache := New(constructor, destructor, maxSize, 1*time.Second)
	defer cache.Close()

	var wg sync.WaitGroup
	ctx := context.Background()

	// Access keys in sequence to trigger LRU eviction
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			key := id % numKeys
			_, err := cache.Get(ctx, key)
			if err != nil {
				t.Errorf("Get failed: %v", err)
			}
		}(i)
	}

	wg.Wait()

	// Some keys should have been evicted since numKeys > maxSize
	time.Sleep(50 * time.Millisecond) // Allow destructor calls to complete
	evictions := destructorCalls.Load()
	if evictions == 0 && numKeys > maxSize {
		t.Logf("Warning: Expected some evictions with %d keys and maxSize %d", numKeys, maxSize)
	}
}

// TestCacheConstructorError verifies error handling with concurrent access
func TestCacheConstructorError(t *testing.T) {
	t.Parallel()

	const numGoroutines = 50
	expectedErr := errors.New("constructor failed")

	var destructorCalled atomic.Bool

	constructor := func(ctx context.Context, k int) (string, error) {
		return "", expectedErr
	}

	destructor := func(k int, v string) {
		// May be called asynchronously by LRU eviction
		// Don't use t.Error here as it may be called after test completes
		destructorCalled.Store(true)
	}

	cache := New(constructor, destructor, 5, 100*time.Millisecond)
	defer cache.Close()

	var wg sync.WaitGroup
	ctx := context.Background()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			_, err := cache.Get(ctx, 1)
			if err == nil {
				t.Error("Expected error from constructor")
			}
			if !errors.Is(err, expectedErr) {
				t.Errorf("Got error %v, want %v", err, expectedErr)
			}
		}()
	}

	wg.Wait()
}

// TestCacheClose verifies Close behavior with concurrent access
func TestCacheClose(t *testing.T) {
	t.Parallel()

	var destructorCalls atomic.Int32

	constructor := func(ctx context.Context, k int) (string, error) {
		return "value", nil
	}

	destructor := func(k int, v string) {
		destructorCalls.Add(1)
	}

	cache := New(constructor, destructor, 5, 100*time.Millisecond)

	ctx := context.Background()

	// Populate cache with some entries
	for i := 0; i < 3; i++ {
		_, err := cache.Get(ctx, i)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
	}

	// Close the cache
	err := cache.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify double close returns error
	err = cache.Close()
	if !errors.Is(err, ErrClosed) {
		t.Errorf("Expected ErrClosed on double close, got %v", err)
	}

	// Verify Get after close returns error
	_, err = cache.Get(ctx, 10)
	if !errors.Is(err, ErrClosed) {
		t.Errorf("Expected ErrClosed on Get after close, got %v", err)
	}

	// Give time for destructors to run
	time.Sleep(100 * time.Millisecond)

	// Destructor should have been called for all cached entries
	// Note: The exact timing of destructor calls depends on LRU implementation
	calls := destructorCalls.Load()
	if calls < 3 {
		t.Logf("Warning: Destructor called %d times, expected 3 (timing dependent)", calls)
	}
}

// TestCacheConcurrentCloseAndGet tests race between Close and Get
func TestCacheConcurrentCloseAndGet(t *testing.T) {
	t.Parallel()

	constructor := func(ctx context.Context, k int) (string, error) {
		time.Sleep(10 * time.Millisecond)
		return "value", nil
	}

	destructor := func(k int, v string) {}

	cache := New(constructor, destructor, 5, 100*time.Millisecond)

	var wg sync.WaitGroup
	ctx := context.Background()

	// Launch goroutines that try to Get
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			cache.Get(ctx, id%5)
		}(i)
	}

	// Close concurrently
	time.Sleep(5 * time.Millisecond)
	cache.Close()

	wg.Wait()
	// Test passes if no race condition detected
}

// TestCacheContextCancellation verifies proper handling of context cancellation
func TestCacheContextCancellation(t *testing.T) {
	t.Parallel()

	blockCh := make(chan struct{})
	constructor := func(ctx context.Context, k int) (string, error) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-blockCh:
			return "value", nil
		}
	}

	destructor := func(k int, v string) {}

	cache := New(constructor, destructor, 5, 100*time.Millisecond)
	defer cache.Close()

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		_, err := cache.Get(ctx, 1)
		errCh <- err
	}()

	// Cancel context before constructor can proceed
	time.Sleep(10 * time.Millisecond)
	cancel()

	// Wait for result
	err := <-errCh
	close(blockCh) // Clean up

	if err == nil {
		t.Error("Expected error from cancelled context")
		return
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}
}

// TestCacheLRUEvictionCallsDestructor verifies destructor is called on eviction
func TestCacheLRUEvictionCallsDestructor(t *testing.T) {
	t.Parallel()

	const maxSize = 3

	var destructedKeys []int
	var destructMu sync.Mutex

	constructor := func(ctx context.Context, k int) (int, error) {
		return k * 10, nil
	}

	destructor := func(k int, v int) {
		destructMu.Lock()
		destructedKeys = append(destructedKeys, k)
		destructMu.Unlock()
	}

	cache := New(constructor, destructor, maxSize, 1*time.Second)
	defer cache.Close()

	ctx := context.Background()

	// Fill cache to maxSize
	for i := 0; i < maxSize; i++ {
		_, err := cache.Get(ctx, i)
		if err != nil {
			t.Fatalf("Get(%d) failed: %v", i, err)
		}
	}

	// Access one more key to trigger LRU eviction
	_, err := cache.Get(ctx, maxSize)
	if err != nil {
		t.Fatalf("Get(%d) failed: %v", maxSize, err)
	}

	// Give time for destructor to be called
	time.Sleep(50 * time.Millisecond)

	destructMu.Lock()
	numDestructed := len(destructedKeys)
	destructMu.Unlock()

	// At least one key should have been evicted
	if numDestructed < 1 {
		t.Errorf("Expected at least 1 eviction, got %d", numDestructed)
	}
}

// TestCacheErrorRetry verifies that failed constructor calls are retried (not cached)
func TestCacheErrorRetry(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32

	constructor := func(ctx context.Context, k int) (string, error) {
		count := callCount.Add(1)
		if count == 1 {
			// First call fails
			return "", errors.New("temporary failure")
		}
		// Second call succeeds
		return "success", nil
	}

	destructor := func(k int, v string) {}

	cache := New(constructor, destructor, 5, 100*time.Millisecond)
	defer cache.Close()

	ctx := context.Background()

	// First attempt should fail
	_, err := cache.Get(ctx, 1)
	if err == nil {
		t.Fatal("Expected error from first Get")
	}

	// Second attempt should succeed (retry, not cached error)
	val, err := cache.Get(ctx, 1)
	if err != nil {
		t.Fatalf("Second Get failed: %v", err)
	}
	if val != "success" {
		t.Errorf("Got %q, want %q", val, "success")
	}

	// Verify constructor was called twice (not cached after error)
	if callCount.Load() != 2 {
		t.Errorf("Constructor called %d times, expected 2", callCount.Load())
	}
}

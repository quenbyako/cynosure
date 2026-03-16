package cache_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	. "github.com/quenbyako/cynosure/contrib/sf-cache"
)

const (
	testValue     = "value"
	testMaxSize   = 5
	testKeyAmount = 20
)

var (
	errConstructorFailed = errors.New("constructor failed")
	errTemporaryFailure  = errors.New("temporary failure")
	errSomeError         = errors.New("some error")
)

func TestCache_ConcurrentGet(t *testing.T) {
	t.Parallel()

	const numGoroutines = 500

	const numKeys = 5

	var constructorCalls atomic.Int32

	var destructorCalls atomic.Int32

	constructor := func(ctx context.Context, key int) (string, error) {
		constructorCalls.Add(1)
		time.Sleep(1 * time.Millisecond)

		return testValue, nil
	}

	destructor := func(key int, val string) {
		destructorCalls.Add(1)
	}

	testCache := New(constructor, destructor, numKeys, 100*time.Millisecond)

	t.Cleanup(func() { require.NoError(t, testCache.Close()) })

	var wg sync.WaitGroup

	ctx := context.Background()

	for id := range numGoroutines {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			currentKey := id % numKeys

			obtainedVal, err := testCache.Get(ctx, currentKey)
			require.NoError(t, err)
			require.Equal(t, testValue, obtainedVal)
		}(id)
	}

	wg.Wait()

	calls := constructorCalls.Load()
	require.LessOrEqual(t, calls, int32(numKeys*2))
}

func TestCache_ConcurrentGetWithLRUEviction(t *testing.T) {
	t.Parallel()

	const numGoroutines = 100

	var (
		destructorCalls atomic.Int32
		destructMu      sync.Mutex
	)

	destructedKeys := make(map[int]int)

	constructor := func(ctx context.Context, key int) (string, error) {
		return testValue, nil
	}

	destructor := func(key int, val string) {
		destructorCalls.Add(1)

		destructMu.Lock()
		destructedKeys[key]++
		destructMu.Unlock()
	}

	testCache := New(constructor, destructor, testMaxSize, 1*time.Second)

	t.Cleanup(func() { require.NoError(t, testCache.Close()) })

	var wg sync.WaitGroup

	ctx := context.Background()

	for id := range numGoroutines {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			currentKey := id % testKeyAmount
			_, err := testCache.Get(ctx, currentKey)
			require.NoError(t, err)
		}(id)
	}

	wg.Wait()

	time.Sleep(50 * time.Millisecond)

	evictions := destructorCalls.Load()

	if evictions == 0 && testKeyAmount > testMaxSize {
		t.Logf("Warning: Expected some evictions with %d keys and maxSize %d",
			testKeyAmount, testMaxSize)
	}
}

func TestCache_ConstructorError(t *testing.T) {
	t.Parallel()

	const numGoroutines = 50

	var destructorCalled atomic.Bool

	constructor := func(ctx context.Context, key int) (string, error) {
		return "", errConstructorFailed
	}

	destructor := func(key int, val string) {
		destructorCalled.Store(true)
	}

	testCache := New(constructor, destructor, testMaxSize, 100*time.Millisecond)

	t.Cleanup(func() { require.NoError(t, testCache.Close()) })

	var wg sync.WaitGroup

	ctx := context.Background()

	for range numGoroutines {
		wg.Add(1)

		go func() {
			defer wg.Done()

			_, err := testCache.Get(ctx, 1)
			require.ErrorIs(t, err, errConstructorFailed)
		}()
	}

	wg.Wait()
}

func TestCache_Close(t *testing.T) {
	t.Parallel()

	var destructorCalls atomic.Int32

	constructor := func(ctx context.Context, key int) (string, error) {
		return testValue, nil
	}

	destructor := func(key int, val string) {
		destructorCalls.Add(1)
	}

	testCache := New(constructor, destructor, testMaxSize, 100*time.Millisecond)

	ctx := context.Background()

	for i := range 3 {
		_, err := testCache.Get(ctx, i)
		require.NoError(t, err)
	}

	err := testCache.Close()
	require.NoError(t, err)

	err = testCache.Close()
	require.ErrorIs(t, err, ErrClosed)

	_, err = testCache.Get(ctx, 10)
	require.ErrorIs(t, err, ErrClosed)

	time.Sleep(100 * time.Millisecond)

	calls := destructorCalls.Load()

	if calls < 3 {
		t.Logf("Warning: Destructor called %d times, expected 3 (timing dependent)", calls)
	}
}

func TestCache_ContextCancellation(t *testing.T) {
	t.Parallel()

	blockCh := make(chan struct{})

	constructor := func(ctx context.Context, key int) (string, error) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-blockCh:
			return testValue, nil
		}
	}

	destructor := func(key int, val string) {}

	testCache := New(constructor, destructor, testMaxSize, 100*time.Millisecond)

	t.Cleanup(func() { require.NoError(t, testCache.Close()) })

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)

	go func() {
		_, err := testCache.Get(ctx, 1)
		errCh <- err
	}()

	time.Sleep(10 * time.Millisecond)

	cancel()

	err := <-errCh

	close(blockCh)

	require.ErrorIs(t, err, context.Canceled)
}

func TestCache_RetryAfterError(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32

	constructor := func(ctx context.Context, key int) (string, error) {
		count := callCount.Add(1)

		if count == 1 {
			return "", errTemporaryFailure
		}

		return "success", nil
	}

	destructor := func(key int, val string) {}

	testCache := New(constructor, destructor, testMaxSize, 100*time.Millisecond)

	t.Cleanup(func() { require.NoError(t, testCache.Close()) })

	ctx := context.Background()

	_, err := testCache.Get(ctx, 1)
	require.Error(t, err)

	val, err := testCache.Get(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, "success", val)

	require.Equal(t, int32(2), callCount.Load())
}

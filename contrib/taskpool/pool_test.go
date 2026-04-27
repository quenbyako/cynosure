package taskpool_test

import (
	"context"
	"runtime"
	"sync"
	"testing"

	"github.com/quenbyako/cynosure/contrib/taskpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskPool_Submit(t *testing.T) {
	t.Parallel()

	type task struct {
		id int
	}

	var (
		mu        sync.Mutex
		processed []int
		wg        sync.WaitGroup
	)

	handler := func(_ context.Context, tsk task) {
		if tsk.id < 0 {
			return
		}

		mu.Lock()

		processed = append(processed, tsk.id)
		mu.Unlock()

		wg.Done()
	}

	pool := taskpool.New(10, handler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start pool in background
	poolErr := make(chan error, 1)

	go func() {
		poolErr <- pool.Run(ctx)
	}()

	// Wait for pool to start
	for !pool.Submit(context.Background(), task{id: -1}) {
		runtime.Gosched()
	}

	// Submit tasks
	numTasks := 5
	wg.Add(numTasks)

	for i := range numTasks {
		ok := pool.Submit(context.Background(), task{id: i})
		require.True(t, ok)
	}

	wg.Wait()

	mu.Lock()
	require.Len(t, processed, numTasks)

	for i := range numTasks {
		require.Contains(t, processed, i)
	}
	mu.Unlock()

	cancel()

	err := <-poolErr
	require.NoError(t, err)
}

func TestTaskPool_ContextPropagation(t *testing.T) {
	t.Parallel()

	type ctxKey string

	const (
		key ctxKey = "test-key"
		val ctxKey = "test-value"
	)

	var (
		capturedVal any
		wg          sync.WaitGroup
	)

	handler := func(ctx context.Context, id int) {
		if id < 0 {
			return
		}

		capturedVal = ctx.Value(key)

		wg.Done()
	}

	pool := taskpool.New(1, handler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	poolErr := make(chan error, 1)

	go func() {
		poolErr <- pool.Run(ctx)
	}()

	// Wait for pool to start
	for !pool.Submit(context.Background(), -1) {
		runtime.Gosched()
	}

	wg.Add(1)

	submitCtx := context.WithValue(context.Background(), key, val)
	pool.Submit(submitCtx, 1)

	wg.Wait()
	require.Equal(t, val, capturedVal)

	cancel()
	require.NoError(t, <-poolErr)
}

func TestTaskPool_Lifecycle(t *testing.T) {
	t.Parallel()

	handler := func(_ context.Context, _ int) {}
	pool := taskpool.New(1, handler)

	// Submit before Run should fail
	assert.False(t, pool.Submit(context.Background(), 1))

	ctx, cancel := context.WithCancel(context.Background())
	poolErr := make(chan error, 1)

	go func() { poolErr <- pool.Run(ctx) }()

	// Wait for pool to start
	for !pool.Submit(context.Background(), 1) {
		runtime.Gosched()
	}

	// Submit while running should succeed
	require.True(t, pool.Submit(context.Background(), 1))

	cancel()
	<-poolErr

	// Submit after shutdown should fail
	assert.False(t, pool.Submit(context.Background(), 1))
}

func TestTaskPool_GracefulShutdown(t *testing.T) {
	t.Parallel()

	taskStarted := make(chan struct{})
	taskCanFinish := make(chan struct{})
	taskFinished := make(chan struct{})

	handler := func(_ context.Context, _ int) {
		close(taskStarted)
		<-taskCanFinish
		close(taskFinished)
	}

	pool := taskpool.New(1, handler)
	ctx, cancel := context.WithCancel(context.Background())
	poolErr := make(chan error, 1)

	go func() {
		poolErr <- pool.Run(ctx)
	}()

	// Wait for pool to start
	for !pool.Submit(context.Background(), 1) {
		runtime.Gosched()
	}

	// Cancel while task is running
	cancel()

	// Signal task it can finish now
	close(taskCanFinish)

	// Run should block until task is finished
	err := <-poolErr
	require.NoError(t, err)

	// Task should be finished now
	select {
	case <-taskFinished:
		// OK
	default:
		t.Fatal("Task should have finished before pool.Run returned")
	}
}

func TestTaskPool_PanicOnDoubleRun(t *testing.T) {
	t.Parallel()

	pool := taskpool.New(1, func(_ context.Context, _ int) {})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := make(chan error, 1)

	go func() {
		err <- pool.Run(ctx)
	}()

	// Wait for pool to start
	for !pool.Submit(context.Background(), 1) {
		runtime.Gosched()
	}

	assert.Panics(t, func() {
		_ = pool.Run(ctx) //nolint:errcheck
	})
}

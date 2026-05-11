package cynosure

import (
	"context"
	"errors"
	"sync"
)

// lifecycle manages a group of tasks that must execute concurrently but require
// a strict, sequential teardown in the exact reverse order of their scheduling.
//
// This pattern is particularly useful for dependency graphs where higher-level
// services (e.g., HTTP servers) must be fully terminated before their
// underlying dependencies (e.g., database connections) receive the cancellation
// signal.
type lifecycle struct {
	mu    sync.Mutex
	tasks []func(context.Context) error
}

func newLifecycle() *lifecycle { return &lifecycle{} }

// schedule appends a new task to the lifecycle. Tasks are executed in parallel
// when run() is called. This method is thread-safe and can be called
// concurrently.
func (l *lifecycle) schedule(task func(context.Context) error) {
	l.mu.Lock()
	l.tasks = append(l.tasks, task)
	l.mu.Unlock()
}

// run executes all scheduled tasks in parallel and blocks until all tasks have
// finished or been canceled.
//
// It provides the following execution and teardown guarantees:
//
//  1. Parallel Execution: All tasks start concurrently in their own goroutines.
//     The total number of spawned goroutines is exactly equal to the number of
//     scheduled tasks.
//
//  2. Fail-Fast: If any single task returns an error, or if the parent ctx is
//     canceled, the lifecycle immediately aborts the wait phase and begins the
//     teardown process.
//
//  3. Strict Reverse-Order Cancellation: During teardown, tasks are canceled
//     sequentially from the last scheduled task down to the first (LIFO).
//     Task N-1 will receive a context cancellation and must fully complete
//     before Task N-2 receives its cancellation signal.
//
// CRITICAL WARNINGS & GOTCHAS:
//
//   - Cooperative Cancellation is Mandatory: The sequential teardown strictly
//     waits (<-done) for a task to exit before canceling the next one. If a
//     task blocks indefinitely and ignores its ctx.Done() signal, the entire
//     teardown process will deadlock.
//
//   - Deadline Drift: Because tasks are canceled sequentially, earlier tasks
//     (e.g., Task 0) might continue running long after the parent ctx has
//     expired. Task 0's context is only canceled after all subsequent tasks
//     have gracefully shut down. Do not rely on the parent context's deadline
//     for strict timing inside individual tasks.
//
//   - Panics: Unrecovered panics inside a task will bypass error assignment.
//     Ensure tasks recover from their own panics if graceful shutdown of the
//     remaining tasks is required.
//
//   - Error Combination: Returns a joined error containing all task failures
//     and/or the parent context cancellation cause. Expected context.Canceled
//     errors from gracefully shutting down tasks are typically filtered out to
//     prevent noise.
func (l *lifecycle) run(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	var funcs []func(context.Context) error
	l.mu.Lock()
	funcs, l.tasks = l.tasks, nil
	l.mu.Unlock()

	n := len(funcs)
	if n == 0 {
		return nil
	}

	tasks := make([]struct {
		cancel context.CancelFunc
		done   chan struct{}
	}, n)

	errs := make([]error, n+1) // +1 for context error
	success := make(chan bool, n)
	defer close(success)

	for i := range n {
		tasks[i].done = make(chan struct{})

		workerCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
		tasks[i].cancel = cancel

		go func(idx int, c context.Context) {
			// note: panic in func drops all application, since there is no
			// recover in spwaned goroutine. THIS MADE BY INTENTION, to prevent
			// unexpected behavior, better to make all app dead instead of
			// handling billion cases and scenarios.
			defer func() {
				success <- errs[idx] == nil
				close(tasks[idx].done)
			}()

			errs[idx] = funcs[idx](c)
		}(i, workerCtx)
	}

waitLoop:
	for range n {
		select {
		case <-ctx.Done():
			errs[n] = context.Cause(ctx)
			break waitLoop

		case ok := <-success:
			if !ok {
				break waitLoop
			}
		}
	}

	for i := n - 1; i >= 0; i-- {
		tasks[i].cancel()
		<-tasks[i].done
	}

	return errors.Join(errs...)
}

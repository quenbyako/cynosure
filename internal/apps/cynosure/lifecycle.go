package cynosure

import (
	"context"
	"errors"
	"fmt"
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
	tasks []func(context.Context) error
	mu    sync.Mutex
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
	if err := context.Cause(ctx); err != nil {
		return fmt.Errorf("parent context error: %w", err)
	}

	l.mu.Lock()
	funcs := l.tasks
	l.tasks = nil
	l.mu.Unlock()

	if len(funcs) == 0 {
		return nil
	}

	return l.executeTasks(ctx, funcs)
}

func (l *lifecycle) executeTasks(ctx context.Context, funcs []func(context.Context) error) error {
	taskCount := len(funcs)
	tasks := l.setupTasks(ctx, taskCount, funcs)
	errs := make([]error, taskCount+1) // +1 for context error

	success := make(chan bool, taskCount)
	defer close(success)

	for i, task := range tasks {
		go func(idx int, workerCtx context.Context, done chan struct{}) { //nolint:contextcheck
			// note: panic in func drops all application, since there is no
			// recover in spawned goroutine. THIS MADE BY INTENTION, to prevent
			// unexpected behavior, better to make all app dead instead of
			// handling billion cases and scenarios.
			defer func() {
				success <- errs[idx] == nil

				close(done)
			}()

			errs[idx] = funcs[idx](workerCtx)
		}(i, task.ctx, task.done)
	}

	l.waitForTasks(ctx, taskCount, success, errs)

	for i := taskCount - 1; i >= 0; i-- {
		tasks[i].cancel()
		<-tasks[i].done
	}

	return errors.Join(errs...)
}

type taskState struct {
	ctx    context.Context //nolint:containedctx
	cancel context.CancelFunc
	done   chan struct{}
}

func (l *lifecycle) setupTasks(
	ctx context.Context, count int, _ []func(context.Context) error,
) []taskState {
	tasks := make([]taskState, count)
	for i := range count {
		tasks[i].done = make(chan struct{})
		//nolint:gosec,fatcontext // see teardown
		tasks[i].ctx, tasks[i].cancel = context.WithCancel(context.WithoutCancel(ctx))
	}

	return tasks
}

func (l *lifecycle) waitForTasks(ctx context.Context, count int, success chan bool, errs []error) {
	for range count {
		select {
		case <-ctx.Done():
			errs[count] = context.Cause(ctx)
			return
		case ok := <-success:
			if !ok {
				return
			}
		}
	}
}

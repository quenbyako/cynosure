// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cache_test

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/quenbyako/cynosure/contrib/sf-cache"
)

type testError struct{}

func (*testError) Error() string { return "error value" }

//nolint:forbidigo // This test specifically examines panic behavior.
func TestSingleflight_PanicErrorUnwrap(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name             string
		panicValue       any
		wrappedErrorType bool
	}{{
		name:             "panicError wraps non-error type",
		panicValue:       "string value",
		wrappedErrorType: false,
	}, {
		name:             "panicError wraps error type",
		panicValue:       &testError{},
		wrappedErrorType: true,
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var recoveredValue any

			group := &Group[struct{}]{}

			func() {
				defer func() {
					recoveredValue = recover()
					t.Logf("after panic(%#v) in group.Do, recoveredValue %#v",
						tc.panicValue, recoveredValue)
				}()

				//nolint:errcheck // Testing panic behavior.
				_, _, _ = group.Do(t.Context(), func(ctx context.Context) (struct{}, error) {
					panic(tc.panicValue)
				})
			}()

			require.NotNil(t, recoveredValue)

			err, ok := recoveredValue.(error)
			require.True(t, ok, "recoveredValue non-error type: %T", recoveredValue)

			if tc.wrappedErrorType {
				require.ErrorIs(t, err, &testError{})
			}
		})
	}
}

func TestSingleflight_Do(t *testing.T) {
	var group Group[string]

	val, err, _ := group.Do(t.Context(), func(context.Context) (string, error) {
		return "bar", nil
	})

	require.Equal(t, "bar", val)
	require.NoError(t, err)
}

func TestSingleflight_DoErr(t *testing.T) {
	var group Group[struct{}]

	_, err, _ := group.Do(t.Context(), func(context.Context) (struct{}, error) {
		return struct{}{}, errSomeError
	})

	require.ErrorIs(t, err, errSomeError)
}

func TestSingleflight_DoDupSuppress(t *testing.T) {
	var (
		group                          Group[string]
		firstCallStarted, allCallsDone sync.WaitGroup
		actualCallCount                int32
	)

	resultChan := make(chan string, 1)

	constructor := func(context.Context) (string, error) {
		if atomic.AddInt32(&actualCallCount, 1) == 1 {
			firstCallStarted.Done()
		}

		val := <-resultChan
		resultChan <- val

		time.Sleep(10 * time.Millisecond)

		if val == "return_error" {
			return "", errSomeError
		}

		return val, nil
	}

	const concurrentCallers = 10

	firstCallStarted.Add(1)

	for range concurrentCallers {
		firstCallStarted.Add(1)
		allCallsDone.Add(1)

		go func() {
			defer allCallsDone.Done()

			firstCallStarted.Done()

			val, err, _ := group.Do(t.Context(), constructor)
			assert.NoError(t, err)
			assert.Equal(t, "bar", val)
		}()
	}

	firstCallStarted.Wait()

	resultChan <- "bar"

	allCallsDone.Wait()

	finalCallCount := atomic.LoadInt32(&actualCallCount)
	require.Positive(t, finalCallCount)
	require.Less(t, finalCallCount, int32(concurrentCallers))
}

func TestSingleflight_Forget(t *testing.T) {
	var group Group[int]

	firstCallStarted := make(chan struct{})
	unblockFirstCall := make(chan struct{})
	firstCallFinished := make(chan struct{})

	go func() {
		//nolint:errcheck // Testing forget behavior.
		_, _, _ = group.Do(t.Context(), func(context.Context) (int, error) {
			close(firstCallStarted)
			<-unblockFirstCall
			close(firstCallFinished)

			return 1, nil
		})
	}()

	<-firstCallStarted
	group.Forget()

	unblockSecondCall := make(chan struct{})
	secondCallDone := make(chan struct{})
	secondCallStarted := make(chan struct{})

	go func() {
		//nolint:errcheck // Testing forget behavior.
		_, _, _ = group.Do(t.Context(), func(ctx context.Context) (int, error) {
			close(secondCallStarted)
			<-unblockSecondCall

			return 2, nil
		})

		close(secondCallDone)
	}()

	<-secondCallStarted
	close(unblockFirstCall)
	<-firstCallFinished

	thirdCallResult := make(chan int, 1)

	go func() {
		//nolint:errcheck // Testing forget behavior.
		res, _, _ := group.Do(t.Context(), func(ctx context.Context) (int, error) {
			return 3, nil
		})

		thirdCallResult <- res

		close(thirdCallResult)
	}()

	close(unblockSecondCall)

	<-secondCallDone

	finalResult := <-thirdCallResult
	require.Equal(t, 2, finalResult)
}

//nolint:forbidigo // Specifically testing panic behavior.
func TestSingleflight_PanicDo(t *testing.T) {
	var group Group[struct{}]

	panicFunction := func(context.Context) (struct{}, error) {
		panic("invalid memory address or nil pointer dereference")
	}

	const totalParallelCalls = 5

	pendingCalls := int32(totalParallelCalls)
	panicsCaught := int32(0)
	testFinished := make(chan struct{})

	for range totalParallelCalls {
		go func() {
			defer func() {
				if caught := recover(); caught != nil {
					t.Logf("Got panic: %v", caught)
					atomic.AddInt32(&panicsCaught, 1)
				}

				if atomic.AddInt32(&pendingCalls, -1) == 0 {
					close(testFinished)
				}
			}()

			//nolint:errcheck // Testing panic behavior.
			_, _, _ = group.Do(t.Context(), panicFunction)
		}()
	}

	select {
	case <-testFinished:
		require.Equal(t, int32(totalParallelCalls), atomic.LoadInt32(&panicsCaught))
	case <-time.After(time.Second):
		t.Fatalf("Do hangs")
	}
}

func TestSingleflight_GoexitDo(t *testing.T) {
	var group Group[struct{}]

	exitFunction := func(context.Context) (struct{}, error) {
		runtime.Goexit()

		return struct{}{}, nil
	}

	const totalParallelCalls = 5

	pendingCalls := int32(totalParallelCalls)
	testFinished := make(chan struct{})

	for range totalParallelCalls {
		go func() {
			var err error

			defer func() {
				if err != nil {
					t.Errorf("Error should be nil, but got: %v", err)
				}

				if atomic.AddInt32(&pendingCalls, -1) == 0 {
					close(testFinished)
				}
			}()

			_, err, _ = group.Do(t.Context(), exitFunction)
		}()
	}

	select {
	case <-testFinished:
	case <-time.After(time.Second):
		t.Fatalf("Do hangs")
	}
}

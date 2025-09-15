// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cache

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type errValue struct{}

func (err *errValue) Error() string {
	return "error value"
}

func TestPanicErrorUnwrap(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name             string
		panicValue       interface{}
		wrappedErrorType bool
	}{
		{
			name:             "panicError wraps non-error type",
			panicValue:       &panicError{value: "string value"},
			wrappedErrorType: false,
		},
		{
			name:             "panicError wraps error type",
			panicValue:       &panicError{value: new(errValue)},
			wrappedErrorType: false,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var recovered interface{}

			group := &group[struct{}]{}

			func() {
				defer func() {
					recovered = recover()
					t.Logf("after panic(%#v) in group.Do, recovered %#v", tc.panicValue, recovered)
				}()

				_, _, _ = group.Do(t.Context(), func(ctx context.Context) (struct{}, error) {
					panic(tc.panicValue)
				})
			}()

			if recovered == nil {
				t.Fatal("expected a non-nil panic value")
			}

			err, ok := recovered.(error)
			if !ok {
				t.Fatalf("recovered non-error type: %T", recovered)
			}

			if !errors.Is(err, new(errValue)) && tc.wrappedErrorType {
				t.Errorf("unexpected wrapped error type %T; want %T", err, new(errValue))
			}
		})
	}
}

func TestDo(t *testing.T) {
	var g group[string]
	v, err, _ := g.Do(t.Context(), func(context.Context) (string, error) { return "bar", nil })
	if got, want := fmt.Sprintf("%v (%T)", v, v), "bar (string)"; got != want {
		t.Errorf("Do = %v; want %v", got, want)
	}
	if err != nil {
		t.Errorf("Do error = %v", err)
	}
}

func TestDoErr(t *testing.T) {
	var g group[struct{}]
	someErr := errors.New("Some error")
	_, err, _ := g.Do(t.Context(), func(context.Context) (struct{}, error) { return struct{}{}, someErr })
	if err != someErr {
		t.Errorf("Do error = %v; want someErr %v", err, someErr)
	}
}

func TestDoDupSuppress(t *testing.T) {
	var g group[string]
	var wg1, wg2 sync.WaitGroup
	c := make(chan string, 1)
	var calls int32
	fn := func(context.Context) (string, error) {
		if atomic.AddInt32(&calls, 1) == 1 {
			// First invocation.
			wg1.Done()
		}
		v := <-c
		c <- v // pump; make available for any future calls

		time.Sleep(10 * time.Millisecond) // let more goroutines enter Do

		return v, nil
	}

	const n = 10
	wg1.Add(1)
	for i := 0; i < n; i++ {
		wg1.Add(1)
		wg2.Add(1)
		go func() {
			defer wg2.Done()
			wg1.Done()
			v, err, _ := g.Do(t.Context(), fn)
			if err != nil {
				t.Errorf("Do error: %v", err)
				return
			}
			if v != "bar" {
				t.Errorf("Do = %T %v; want %q", v, v, "bar")
			}
		}()
	}
	wg1.Wait()
	// At least one goroutine is in fn now and all of them have at
	// least reached the line before the Do.
	c <- "bar"
	wg2.Wait()
	if got := atomic.LoadInt32(&calls); got <= 0 || got >= n {
		t.Errorf("number of calls = %d; want over 0 and less than %d", got, n)
	}
}

// Test that singleflight behaves correctly after Forget called.
// See https://github.com/golang/go/issues/31420
func TestForget(t *testing.T) {
	var g group[int]

	var (
		firstStarted  = make(chan struct{})
		unblockFirst  = make(chan struct{})
		firstFinished = make(chan struct{})
	)

	go func() {
		g.Do(t.Context(), func(context.Context) (i int, e error) {
			close(firstStarted)
			<-unblockFirst
			close(firstFinished)
			return
		})
	}()
	<-firstStarted
	g.Forget()

	unblockSecond := make(chan struct{})
	secondResult := make(chan struct{})
	go func() {
		g.Do(t.Context(), func(ctx context.Context) (i int, e error) { <-unblockSecond; return 2, nil })
		close(secondResult)
	}()

	close(unblockFirst)
	<-firstFinished

	var thirdResult chan int
	go func() {
		res, _, _ := g.Do(t.Context(), func(ctx context.Context) (i int, e error) { return 3, nil })
		thirdResult <- res
		close(thirdResult)
	}()

	close(unblockSecond)
	<-secondResult
	r := <-thirdResult
	if r != 2 {
		t.Errorf("We should receive result produced by second call, expected: 2, got %d", r)
	}
}

// Test singleflight behaves correctly after Do panic.
// See https://github.com/golang/go/issues/41133
func TestPanicDo(t *testing.T) {
	var g group[struct{}]
	fn := func(context.Context) (struct{}, error) {
		panic("invalid memory address or nil pointer dereference")
	}

	const n = 5
	waited := int32(n)
	panicCount := int32(0)
	done := make(chan struct{})
	for i := 0; i < n; i++ {
		go func() {
			defer func() {
				if err := recover(); err != nil {
					t.Logf("Got panic: %v\n%s", err, debug.Stack())
					atomic.AddInt32(&panicCount, 1)
				}

				if atomic.AddInt32(&waited, -1) == 0 {
					close(done)
				}
			}()

			g.Do(t.Context(), fn)
		}()
	}

	select {
	case <-done:
		if panicCount != n {
			t.Errorf("Expect %d panic, but got %d", n, panicCount)
		}
	case <-time.After(time.Second):
		t.Fatalf("Do hangs")
	}
}

func TestGoexitDo(t *testing.T) {
	var g group[struct{}]
	fn := func(context.Context) (struct{}, error) {
		runtime.Goexit()
		return struct{}{}, nil
	}

	const n = 5
	waited := int32(n)
	done := make(chan struct{})
	for i := 0; i < n; i++ {
		go func() {
			var err error
			defer func() {
				if err != nil {
					t.Errorf("Error should be nil, but got: %v", err)
				}
				if atomic.AddInt32(&waited, -1) == 0 {
					close(done)
				}
			}()
			_, err, _ = g.Do(t.Context(), fn)
		}()
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("Do hangs")
	}
}

func executable(t testing.TB) string {
	exe, err := os.Executable()
	if err != nil {
		t.Skipf("skipping: test executable not found")
	}

	// Control case: check whether exec.Command works at all.
	// (For example, it might fail with a permission error on iOS.)
	cmd := exec.Command(exe, "-test.list=^$")
	cmd.Env = []string{}
	if err := cmd.Run(); err != nil {
		t.Skipf("skipping: exec appears not to work on %s: %v", runtime.GOOS, err)
	}

	return exe
}

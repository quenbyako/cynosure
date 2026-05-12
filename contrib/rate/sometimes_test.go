// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rate_test

import (
	"fmt"
	"math"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func ExampleSometimes_once() {
	// The zero value of Sometimes behaves like sync.Once, though less efficiently.
	var sometimes rate.Sometimes
	sometimes.Do(func() { fmt.Println("1") })
	sometimes.Do(func() { fmt.Println("2") })
	sometimes.Do(func() { fmt.Println("3") })
	// Output:
	// 1
}

func ExampleSometimes_first() {
	sometimes := rate.Sometimes{First: 2}
	sometimes.Do(func() { fmt.Println("1") })
	sometimes.Do(func() { fmt.Println("2") })
	sometimes.Do(func() { fmt.Println("3") })
	// Output:
	// 1
	// 2
}

func ExampleSometimes_every() {
	sometimes := rate.Sometimes{Every: 2}
	sometimes.Do(func() { fmt.Println("1") })
	sometimes.Do(func() { fmt.Println("2") })
	sometimes.Do(func() { fmt.Println("3") })
	// Output:
	// 1
	// 3
}

func ExampleSometimes_interval() {
	sometimes := rate.Sometimes{Interval: 1 * time.Second}
	sometimes.Do(func() { fmt.Println("1") })
	sometimes.Do(func() { fmt.Println("2") })
	time.Sleep(1 * time.Second)
	sometimes.Do(func() { fmt.Println("3") })
	// Output:
	// 1
	// 3
}

func ExampleSometimes_mix() {
	sometimes := rate.Sometimes{
		First:    2,
		Every:    2,
		Interval: 2 * time.Second,
	}
	sometimes.Do(func() { fmt.Println("1 (First:2)") })
	sometimes.Do(func() { fmt.Println("2 (First:2)") })
	sometimes.Do(func() { fmt.Println("3 (Every:2)") })
	time.Sleep(2 * time.Second)
	sometimes.Do(func() { fmt.Println("4 (Interval)") })
	sometimes.Do(func() { fmt.Println("5 (Every:2)") })
	sometimes.Do(func() { fmt.Println("6") })
	// Output:
	// 1 (First:2)
	// 2 (First:2)
	// 3 (Every:2)
	// 4 (Interval)
	// 5 (Every:2)
}

func TestSometimesZero(t *testing.T) {
	sometimes := rate.Sometimes{Interval: 0}
	sometimes.Do(func() {})
	sometimes.Do(func() {})
}

func TestSometimesMax(t *testing.T) {
	sometimes := rate.Sometimes{Interval: math.MaxInt64}
	sometimes.Do(func() {})
	sometimes.Do(func() {})
}

func TestSometimesNegative(t *testing.T) {
	sometimes := rate.Sometimes{Interval: -1}
	sometimes.Do(func() {})
	sometimes.Do(func() {})
}

func BenchmarkSometimes(b *testing.B) {
	b.Run("no-interval", func(b *testing.B) {
		sometimes := rate.Sometimes{Every: 10}
		for range b.N {
			sometimes.Do(func() {})
		}
	})
	b.Run("with-interval", func(b *testing.B) {
		sometimes := rate.Sometimes{Interval: time.Second}
		for range b.N {
			sometimes.Do(func() {})
		}
	})
}

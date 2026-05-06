// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package rate provides a rate limiter.
package rate

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
)

// Limit defines the maximum frequency of some events.
// Limit is represented as number of events per second.
// A zero Limit allows no events.
type Limit float64

// Inf is the infinite rate limit; it allows all events (even if bucket is zero).
const (
	Inf = Limit(math.MaxFloat64)
)

// Every converts a minimum time interval between events to a Limit.
func Every(interval time.Duration) Limit {
	if interval <= 0 {
		return Inf
	}

	if interval == InfDuration {
		return 0
	}

	return 1 / Limit(interval.Seconds())
}

func rateAsLimit(period time.Duration, bucket int) Limit {
	if period <= 0 {
		return Inf
	}

	if period == InfDuration {
		return 0
	}

	return Limit(float64(bucket) / period.Seconds())
}

// A Limiter controls how frequently events are allowed to happen. It implements
// a "token bucket" of size b, initially full and refilled such that the entire
// bucket is replenished every period p.
//
// Informally, the Limiter ensures that no more than b tokens are consumed in
// any rolling window of duration p. The effective rate is b/p tokens per
// second. As a special case, if the rate is Inf, b is ignored. See
// https://en.wikipedia.org/wiki/Token_bucket for more about token buckets.
//
// The zero value is a valid Limiter, but it will reject all events. Use
// NewLimiter to create non-zero Limiters.
//
// Behavior:
//   - Initially, the bucket is full (contains b tokens).
//   - When a token is consumed, the bucket level decreases.
//   - Over time, tokens are refilled at a constant rate.
//   - If the bucket is empty, subsequent requests must wait for refill.
//   - This implementation supports "debt" (negative balance) via SoftAllow/SoftReserve,
//     allowing a bucket to complete if it started with a positive balance.
//   - A "Penalty Ceiling" (maxWait) can be set to cap the maximum debt/wait time.
//
// Limiter has three main methods: Allow, Reserve, and Wait.
//   - Allow: Reports if a token can be consumed NOW. Returns false if not.
//   - Reserve: Reserves a token and returns a Reservation indicating when it can be used.
//   - Wait: Blocks until a token is available or the context is canceled.
//
// The "Soft" variants (SoftAllow, SoftReserve) permit "overdrafts": if the
// balance is positive, the entire request is allowed even if it exceeds current
// tokens, pushing the bucket into a negative state (debt).
//
// Limiter is safe for simultaneous use by multiple goroutines.
type Limiter struct {
	// last is the last time the limiter's tokens field was updated
	last time.Time
	// lastEvent is the latest time of a rate-limited event (past or future)
	lastEvent time.Time
	limit     Limit
	bucket    int
	// tokens is the current number of tokens in the bucket.
	tokens float64
	// minTokens is the minimum number of tokens allowed (Penalty Ceiling). This
	// defines the maximum "debt" the bucket can accrue. If the debt exceeds
	// this amount, the wait time is capped.
	minTokens float64
	mu        sync.Mutex
}

// Limit returns the maximum overall event rate.
func (lim *Limiter) Limit() Limit {
	lim.mu.Lock()
	defer lim.mu.Unlock()

	return lim.limit
}

// Burst returns the maximum bucket size. Burst is the maximum number of tokens
// that can be consumed in a single call to Allow, Reserve, or Wait, so higher
// Burst values allow more events to happen at once. A zero Burst allows no
// events, unless limit == Inf.
func (lim *Limiter) Burst() int {
	lim.mu.Lock()
	defer lim.mu.Unlock()

	return lim.bucket
}

// TokensAt returns the number of tokens available at time t.
func (lim *Limiter) TokensAt(t time.Time) float64 {
	lim.mu.Lock()
	defer lim.mu.Unlock()

	tokens := lim.advance(t) // does not mutate lim

	return tokens
}

// Tokens returns the number of tokens available now.
func (lim *Limiter) Tokens() float64 {
	return lim.TokensAt(time.Now())
}

// NewLimiter returns a new Limiter that allows events up to rate bucket/period
// and permits buckets of at most bucket tokens.
func NewLimiter(period time.Duration, bucket int) *Limiter {
	return NewLimiterWithMaxWait(period, bucket, 0)
}

// NewLimiterWithMaxWait returns a new Limiter with a maximum wait time (Penalty
// Ceiling).
func NewLimiterWithMaxWait(period time.Duration, bucket int, maxWait time.Duration) *Limiter {
	limit := rateAsLimit(period, bucket)

	minTokens := -math.MaxFloat64
	if maxWait > 0 {
		minTokens = -limit.TokensFromDuration(maxWait)
	}

	return &Limiter{
		limit:     limit,
		bucket:    bucket,
		tokens:    float64(bucket),
		minTokens: minTokens,
	}
}

// Allow reports whether an event may happen now.
func (lim *Limiter) Allow() bool {
	return lim.AllowN(time.Now(), 1)
}

// AllowN reports whether n events may happen at time t.
// Use this method if you intend to drop / skip events that exceed the rate limit.
// Otherwise use Reserve or Wait.
func (lim *Limiter) AllowN(t time.Time, n int) bool {
	return lim.reserveN(t, n, 0, false, false).ok
}

// SoftAllow reports whether an event may happen now with penalty.
func (lim *Limiter) SoftAllow() bool {
	return lim.SoftAllowN(time.Now(), 1)
}

// SoftAllowN reports whether n events may happen at time t with penalty. It
// allows an "overdraft": if tokensBefore > 0, the request is granted even if it
// consumes more than available tokens, resulting in a negative balance. This is
// useful for allowing a logical bucket (e.g. one full LLM response) to finish
// even if it slightly exceeds the rate, while delaying future requests.
func (lim *Limiter) SoftAllowN(t time.Time, n int) bool {
	return lim.reserveN(t, n, 0, true, false).ok
}

// A Reservation holds information about events that are permitted by a Limiter
// to happen after a delay. A Reservation may be canceled, which may enable the
// Limiter to permit additional events.
type Reservation struct {
	timeToAct time.Time
	lim       *Limiter
	tokens    int
	// This is the Limit at reservation time, it can change later.
	limit Limit
	ok    bool
}

// OK returns whether the limiter can provide the requested number of tokens
// within the maximum wait time.  If OK is false, Delay returns InfDuration, and
// Cancel does nothing.
func (r *Reservation) OK() bool {
	return r.ok
}

func (r *Reservation) RetryAt() time.Time {
	return r.timeToAct
}

// Delay is shorthand for DelayFrom(time.Now()).
func (r *Reservation) Delay() time.Duration {
	return r.DelayFrom(time.Now())
}

// InfDuration is the duration returned by Delay when a Reservation is not OK.
const InfDuration = time.Duration(math.MaxInt64)

// DelayFrom returns the duration for which the reservation holder must wait
// before taking the reserved action.  Zero duration means act immediately.
// InfDuration means the limiter cannot grant the tokens requested in this
// Reservation within the maximum wait time.
func (r *Reservation) DelayFrom(t time.Time) time.Duration {
	if !r.ok {
		return InfDuration
	}

	delay := r.timeToAct.Sub(t)
	if delay < 0 {
		return 0
	}

	return delay
}

// Cancel is shorthand for CancelAt(time.Now()).
func (r *Reservation) Cancel() {
	r.CancelAt(time.Now())
}

// CancelAt indicates that the reservation holder will not perform the reserved action
// and reverses the effects of this Reservation on the rate limit as much as possible,
// considering that other reservations may have already been made.
func (r *Reservation) CancelAt(t time.Time) {
	if !r.ok {
		return
	}

	r.lim.mu.Lock()
	defer r.lim.mu.Unlock()

	if r.lim.limit == Inf || r.tokens == 0 || r.timeToAct.Before(t) {
		return
	}

	// calculate tokens to restore
	// The duration between lim.lastEvent and r.timeToAct tells us how many tokens were reserved
	// after r was obtained. These tokens should not be restored.
	elapsed := r.lim.lastEvent.Sub(r.timeToAct)

	restoreTokens := float64(r.tokens) - r.limit.TokensFromDuration(elapsed)
	if restoreTokens <= 0 {
		return
	}
	// advance time to now
	tokens := r.lim.advance(t)
	// calculate new number of tokens
	tokens += restoreTokens
	if bucket := float64(r.lim.bucket); tokens > bucket {
		tokens = bucket
	}
	// update state
	r.lim.last = t

	r.lim.tokens = tokens
	if r.timeToAct.Equal(r.lim.lastEvent) {
		prevEvent := r.timeToAct.Add(r.limit.DurationFromTokens(float64(-r.tokens)))
		if !prevEvent.Before(t) {
			r.lim.lastEvent = prevEvent
		}
	}
}

// Reserve is shorthand for ReserveN(time.Now(), 1).
func (lim *Limiter) Reserve() *Reservation {
	return lim.ReserveN(time.Now(), 1)
}

// ReserveN returns a Reservation that indicates how long the caller must wait
// before n events happen. The Limiter takes this Reservation into account when
// allowing future events. The returned Reservation’s OK() method returns false
// if n exceeds the Limiter's bucket size. Usage example:
//
//	r := lim.ReserveN(time.Now(), 1)
//	if !r.OK() {
//	  // Not allowed to act! Did you remember to set lim.bucket to be > 0 ?
//	  return
//	}
//	time.Sleep(r.Delay())
//	Act()
//
// Use this method if you wish to wait and slow down in accordance with the rate
// limit without dropping events. If you need to respect a deadline or cancel
// the delay, use Wait instead. To drop or skip events exceeding rate limit, use
// Allow instead.
func (lim *Limiter) ReserveN(t time.Time, n int) *Reservation {
	r := lim.reserveN(t, n, InfDuration, false, false)
	return &r
}

// SoftReserveN is like ReserveN but allows an "overdraft" if the current
// balance is positive. If tokensBefore > 0, the reservation is granted with
// timeToAct = now, but the bucket goes into debt, delaying subsequent
// reservations.
func (lim *Limiter) SoftReserveN(t time.Time, n int) *Reservation {
	r := lim.reserveN(t, n, InfDuration, true, false)
	return &r
}

// ForceReserveN is like ReserveN but it ALWAYS succeeds, even if the current
// balance is negative or zero. This is used for "settlement" where we must
// record usage regardless of limits. The Penalty Ceiling is still applied
// to the resulting debt.
func (lim *Limiter) ForceReserveN(t time.Time, n int) *Reservation {
	r := lim.reserveN(t, n, InfDuration, false, true)
	return &r
}

// Wait is shorthand for WaitN(ctx, 1).
func (lim *Limiter) Wait(ctx context.Context) (err error) {
	return lim.WaitN(ctx, 1)
}

// WaitN blocks until lim permits n events to happen.
// It returns an error if n exceeds the Limiter's bucket size, the Context is
// canceled, or the expected wait time exceeds the Context's Deadline.
// The bucket limit is ignored if the rate limit is Inf.
func (lim *Limiter) WaitN(ctx context.Context, n int) (err error) {
	// The test code calls lim.wait with a fake timer generator.
	// This is the real timer generator.
	newTimer := func(d time.Duration) (<-chan time.Time, func() bool, func()) {
		timer := time.NewTimer(d)
		return timer.C, timer.Stop, func() {}
	}

	return lim.wait(ctx, n, time.Now(), newTimer)
}

// wait is the internal implementation of WaitN.
func (lim *Limiter) wait(
	ctx context.Context,
	n int,
	t time.Time,
	newTimer func(d time.Duration) (<-chan time.Time, func() bool, func()),
) error {
	if err := lim.checkWaitN(n); err != nil {
		return err
	}
	// Check if ctx is already cancelled
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	// Determine wait limit
	waitLimit := InfDuration
	if deadline, ok := ctx.Deadline(); ok {
		waitLimit = deadline.Sub(t)
	}
	// Reserve
	reservation := lim.reserveN(t, n, waitLimit, false, false)
	if !reservation.ok {
		return fmt.Errorf("rate: Wait(n=%d) would exceed context deadline", n)
	}
	// Wait if necessary
	delay := reservation.DelayFrom(t)
	if delay == 0 {
		return nil
	}

	return lim.waitWithTimer(ctx, delay, reservation, newTimer)
}

func (lim *Limiter) checkWaitN(n int) error {
	lim.mu.Lock()
	defer lim.mu.Unlock()

	if n > lim.bucket && lim.limit != Inf {
		return fmt.Errorf("rate: Wait(n=%d) exceeds limiter's bucket %d", n, lim.bucket)
	}

	return nil
}

func (lim *Limiter) waitWithTimer(
	ctx context.Context,
	delay time.Duration,
	reservation Reservation,
	newTimer func(d time.Duration) (<-chan time.Time, func() bool, func()),
) error {
	ch, stop, advance := newTimer(delay)
	defer stop()

	advance() // only has an effect when testing

	select {
	case <-ch:
		// We can proceed.
		return nil
	case <-ctx.Done():
		// Context was canceled before we could proceed.  Cancel the
		// reservation, which may permit other events to proceed sooner.
		reservation.Cancel()
		return ctx.Err()
	}
}

// SetLimit is shorthand for SetLimitAt(time.Now(), period).
func (lim *Limiter) SetLimit(period time.Duration) {
	lim.SetLimitAt(time.Now(), period)
}

// SetLimitAt sets a new Limit for the limiter. The new Limit, and Burst, may be violated
// or underutilized by those which reserved (using Reserve or Wait) but did not yet act
// before SetLimitAt was called.
func (lim *Limiter) SetLimitAt(t time.Time, period time.Duration) {
	lim.mu.Lock()
	defer lim.mu.Unlock()

	tokens := lim.advance(t)

	lim.last = t
	lim.tokens = tokens

	lim.limit = Every(period)
	if period > 0 {
		lim.limit = Limit(float64(lim.bucket) / period.Seconds())
	}
}

// SetBurst is shorthand for SetBurstAt(time.Now(), newBurst).
func (lim *Limiter) SetBurst(newBurst int) {
	lim.SetBurstAt(time.Now(), newBurst)
}

// SetBurstAt sets a new bucket size for the limiter.
func (lim *Limiter) SetBurstAt(t time.Time, newBurst int) {
	lim.mu.Lock()
	defer lim.mu.Unlock()

	tokens := lim.advance(t)

	lim.last = t
	lim.tokens = tokens
	lim.bucket = newBurst
}

// reserveN is a helper method for AllowN, ReserveN, and WaitN. maxFutureReserve
// specifies the maximum reservation wait duration allowed. reserveN returns
// Reservation, not *Reservation, to avoid allocation in AllowN and WaitN.
func (lim *Limiter) reserveN(t time.Time, n int, maxFutureReserve time.Duration, overdraft, force bool) Reservation {
	lim.mu.Lock()
	defer lim.mu.Unlock()

	if lim.limit == Inf {
		return Reservation{
			ok:        true,
			lim:       lim,
			tokens:    n,
			timeToAct: t,
			limit:     0,
		}
	}

	tokensBefore := lim.advance(t)
	canOverdraft := force || (overdraft && tokensBefore > 0)

	// Calculate the remaining number of tokens resulting from the request.
	tokensAfter := tokensBefore - float64(n)

	// Calculate the wait duration
	waitDuration := lim.calculateWait(tokensAfter, canOverdraft)

	// Decide result
	ok := force || (waitDuration <= maxFutureReserve && (n <= lim.bucket || canOverdraft))
	// Prepare reservation
	reservation := Reservation{
		ok:        ok,
		lim:       lim,
		limit:     lim.limit,
		tokens:    0,
		timeToAct: t,
	}

	if ok {
		reservation.tokens = n
		reservation.timeToAct = t.Add(waitDuration)

		// Update state
		lim.last = t
		lim.tokens = tokensAfter
		lim.lastEvent = reservation.timeToAct
	}

	return reservation
}

func (lim *Limiter) calculateWait(tokensAfter float64, canOverdraft bool) time.Duration {
	if tokensAfter >= 0 || canOverdraft {
		return 0
	}

	return lim.limit.DurationFromTokens(-tokensAfter)
}

// advance calculates and returns an updated number of tokens for lim
// resulting from the passage of time.
// lim is not changed.
// advance requires that lim.mu is held.
func (lim *Limiter) advance(t time.Time) (newTokens float64) {
	last := lim.last
	if t.Before(last) {
		last = t
	}

	// Calculate the new number of tokens, due to time that passed.
	elapsed := t.Sub(last)
	delta := lim.limit.TokensFromDuration(elapsed)
	// tokens may be no more than bucket limit
	tokens := min(float64(lim.bucket), lim.tokens+delta)
	tokens = max(lim.minTokens, tokens)

	return tokens
}

// DurationFromTokens is a unit conversion function from the number of tokens to the duration
// of time it takes to accumulate them at a rate of limit tokens per second.
func (limit Limit) DurationFromTokens(tokens float64) time.Duration {
	if limit <= 0 {
		return InfDuration
	}

	duration := (tokens / float64(limit)) * float64(time.Second)

	// Cap the duration to the maximum representable int64 value, to avoid overflow.
	if duration > float64(math.MaxInt64) {
		return InfDuration
	}

	return time.Duration(duration)
}

// TokensFromDuration is a unit conversion function from a time duration to the number of tokens
// which could be accumulated during that duration at a rate of limit tokens per second.
func (limit Limit) TokensFromDuration(d time.Duration) float64 {
	if limit <= 0 {
		return 0
	}

	return d.Seconds() * float64(limit)
}

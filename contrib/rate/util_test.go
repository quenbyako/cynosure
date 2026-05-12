//nolint:testpackage // helpers for testing purposes
package rate

import (
	"context"
	"time"
)

func (lim *Limiter) TokensRaw() float64   { return lim.tokens }
func (lim *Limiter) Last() time.Time      { return lim.last }
func (lim *Limiter) LastEvent() time.Time { return lim.lastEvent }

func (lim *Limiter) WaitRaw(
	ctx context.Context,
	tokens int,
	now time.Time,
	newTimer func(d time.Duration) (<-chan time.Time, func() bool, func()),
) error {
	return lim.wait(ctx, tokens, now, newTimer)
}

func (lim *Limiter) ReserveNRaw(
	now time.Time, n int, maxFutureReserve time.Duration, overdraft, force bool,
) Reservation {
	return lim.reserveN(now, n, maxFutureReserve, overdraft, force)
}

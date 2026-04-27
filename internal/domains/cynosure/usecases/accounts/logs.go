package accounts

import (
	"context"
)

type LogCallbacks interface {
	AccountUsecasePoolNotRunning(ctx context.Context)
}

type NoOpLogCallbacks struct{}

var _ LogCallbacks = NoOpLogCallbacks{}

func (n NoOpLogCallbacks) AccountUsecasePoolNotRunning(ctx context.Context) {}

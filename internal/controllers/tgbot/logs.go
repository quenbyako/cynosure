package tgbot

import (
	"context"
)

type LogCallbacks interface {
	ProcessMessageIssue(ctx context.Context, channelID int64, err error)
}

type NoOpLogCallbacks struct{}

var _ LogCallbacks = NoOpLogCallbacks{}

func (n NoOpLogCallbacks) ProcessMessageIssue(ctx context.Context, channelID int64, err error) {}

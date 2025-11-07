package tgbot

import (
	"context"
)

// T008: Enhanced structured logging interface for message processing observability
type LogCallbacks interface {
	ProcessMessageStart(ctx context.Context, channelID int64, messageText string)
	ProcessMessageSuccess(ctx context.Context, channelID int64, duration string)
	ProcessMessageIssue(ctx context.Context, channelID int64, err error)
}

type NoOpLogCallbacks struct{}

var _ LogCallbacks = NoOpLogCallbacks{}

func (n NoOpLogCallbacks) ProcessMessageStart(ctx context.Context, channelID int64, messageText string) {
}
func (n NoOpLogCallbacks) ProcessMessageSuccess(ctx context.Context, channelID int64, duration string) {
}
func (n NoOpLogCallbacks) ProcessMessageIssue(ctx context.Context, channelID int64, err error) {}

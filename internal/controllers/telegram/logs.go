package telegram

import (
	"context"
)

type LogCallbacks interface {
	ProcessMessageStart(ctx context.Context, channelID int, messageText string)
	ProcessMessageSuccess(ctx context.Context, channelID int, duration string)
	ProcessMessageIssue(ctx context.Context, channelID int, err error)
}

type NoOpLogCallbacks struct{}

var _ LogCallbacks = NoOpLogCallbacks{}

func (n NoOpLogCallbacks) ProcessMessageStart(ctx context.Context, channelID int, messageText string) {
}

func (n NoOpLogCallbacks) ProcessMessageSuccess(ctx context.Context, channelID int, duration string) {
}
func (n NoOpLogCallbacks) ProcessMessageIssue(ctx context.Context, channelID int, err error) {}

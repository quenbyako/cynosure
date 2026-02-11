package gemini

import (
	"context"
)

type LogCallbacks interface {
	GeminiStreamStarted(ctx context.Context, model string, toolCount int)
}

type NoOpLogCallbacks struct{}

var _ LogCallbacks = NoOpLogCallbacks{}

func (n NoOpLogCallbacks) GeminiStreamStarted(ctx context.Context, model string, toolCount int) {}

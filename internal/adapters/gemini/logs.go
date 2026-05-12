package gemini

import (
	"context"
)

type LogCallbacks interface {
	GeminiStreamStarted(ctx context.Context, model string, toolCount int)
	TokenCountMismatch(ctx context.Context, model string, expected, got uint32)
}

type NoOpLogCallbacks struct{}

var _ LogCallbacks = NoOpLogCallbacks{}

func (n NoOpLogCallbacks) GeminiStreamStarted(ctx context.Context, model string, toolCount int) {}

func (n NoOpLogCallbacks) TokenCountMismatch(
	ctx context.Context, model string, expected, got uint32,
) {
}

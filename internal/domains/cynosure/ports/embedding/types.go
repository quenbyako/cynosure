package embedding

import (
	"context"
)

// PreflightFunc is a function that checks if a request can be processed. It is
// used to check for rate limits, token limits, etc. If the check fails, the
// request will be aborted.
type PreflightFunc func(ctx context.Context, modelName string, inputTokens int) error

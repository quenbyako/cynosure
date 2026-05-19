//go:build !vcr_record

package vcrtest

import (
	"testing"

	"gopkg.in/dnaeon/go-vcr.v4/pkg/recorder"
)

// Mode returns the VCR mode for replay (default).
func Mode(*testing.T) recorder.Mode { return recorder.ModeReplayOnly }

// GeminiKey returns a dummy key for replay mode.
func GeminiKey(*testing.T) string { return "dummy-key" }

// OpenRouterKey returns a dummy OpenRouter key for replay mode.
func OpenRouterKey(*testing.T) string { return "dummy-key" }

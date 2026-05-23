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

// OryEndpoint returns a dummy Ory API endpoint for replay mode.
func OryEndpoint(*testing.T) string { return "https://dummy.internal" }

// OryAdminKey returns a dummy Ory Admin key for replay mode.
func OryAdminKey(*testing.T) string { return "dummy-ory-admin-key" }

// OryClientID returns a dummy Ory Client ID for replay mode.
func OryClientID(*testing.T) string { return "dummy-ory-client-id" }

// OryClientSecret returns a dummy Ory Client Secret for replay mode.
func OryClientSecret(*testing.T) string { return "dummy-ory-client-secret" }

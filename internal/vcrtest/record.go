//go:build vcr_record

package vcrtest

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/dnaeon/go-vcr.v4/pkg/recorder"
)

const (
	geminiAPIKeyEnv = "CYNOSURE_TEST_GEMINI_API_KEY"
)

// Mode returns the VCR mode for recording.
func Mode(t *testing.T) recorder.Mode {
	return recorder.ModeRecordOnly
}

// GeminiKey returns the real API key from the environment.
// It panics if the key is missing to prevent accidental empty recordings.
func GeminiKey(t *testing.T) string {
	k := strings.TrimSpace(os.Getenv(geminiAPIKeyEnv))
	if k == "" {
		t.Fatalf("VCR recording requested via -tags=vcr_record, but %s is empty", geminiAPIKeyEnv)
	}
	return k
}

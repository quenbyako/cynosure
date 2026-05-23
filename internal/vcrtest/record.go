//go:build vcr_record

package vcrtest

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/dnaeon/go-vcr.v4/pkg/recorder"
)

const (
	geminiAPIKeyEnv    = "CYNOSURE_TEST_GEMINI_API_KEY"
	oryEndpoint        = "CYNOSURE_TEST_ORY_ENDPOINT"
	oryAdminKeyEnv     = "CYNOSURE_TEST_ORY_ADMIN_KEY"
	oryClientIDEnv     = "CYNOSURE_TEST_ORY_CLIENT_ID"
	oryClientSecretEnv = "CYNOSURE_TEST_ORY_CLIENT_SECRET"
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

// OpenRouterKey returns the real OpenRouter API key from the environment.
func OpenRouterKey(t *testing.T) string {
	k := strings.TrimSpace(os.Getenv("CYNOSURE_TEST_OPENROUTER_API_KEY"))
	if k == "" {
		t.Fatalf("VCR recording requested via -tags=vcr_record, but CYNOSURE_TEST_OPENROUTER_API_KEY is empty")
	}
	return k
}

// OryEndpoint returns the real Ory API endpoint from the environment.
func OryEndpoint(t *testing.T) string {
	k := strings.TrimSpace(os.Getenv(oryEndpoint))
	if k == "" {
		t.Fatalf("VCR recording requested via -tags=vcr_record, but %s is empty", oryEndpoint)
	}
	return k
}

// OryAdminKey returns the real Ory Admin API key from the environment.
func OryAdminKey(t *testing.T) string {
	k := strings.TrimSpace(os.Getenv(oryAdminKeyEnv))
	if k == "" {
		t.Fatalf("VCR recording requested via -tags=vcr_record, but %s is empty", oryAdminKeyEnv)
	}
	return k
}

// OryClientID returns the real Ory Client ID from the environment.
func OryClientID(t *testing.T) string {
	k := strings.TrimSpace(os.Getenv(oryClientIDEnv))
	if k == "" {
		t.Fatalf("VCR recording requested via -tags=vcr_record, but %s is empty", oryClientIDEnv)
	}
	return k
}

// OryClientSecret returns the real Ory Client Secret from the environment.
func OryClientSecret(t *testing.T) string {
	k := strings.TrimSpace(os.Getenv(oryClientSecretEnv))
	if k == "" {
		t.Fatalf("VCR recording requested via -tags=vcr_record, but %s is empty", oryClientSecretEnv)
	}
	return k
}

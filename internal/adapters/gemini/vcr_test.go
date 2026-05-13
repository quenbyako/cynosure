package gemini_test

import (
	"embed"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/dnaeon/go-vcr.v4/pkg/cassette"
	"gopkg.in/dnaeon/go-vcr.v4/pkg/recorder"

	"github.com/quenbyako/cynosure/internal/adapters/gemini"
	"github.com/quenbyako/cynosure/internal/vcrtest"
)

//go:embed testdata/*
var testdata embed.FS

func vcrModel(t *testing.T, cassetteName string) (model *gemini.GeminiModel, stop func() error) {
	t.Helper()

	rec := vcrtest.New(t, testdata, cassetteName,
		recorder.WithHook(func(i *cassette.Interaction) error {
			i.Request.Headers.Set("X-Goog-Api-Key", "REDACTED")
			i.Request.Headers.Set("Authorization", "REDACTED")
			i.Response.Headers.Set("Set-Cookie", "REDACTED")

			return nil
		}, recorder.BeforeSaveHook),
	)
	key := vcrtest.GeminiKey(t)

	gem, err := gemini.New(t.Context(), staticSecretGetter(key),
		append(geminiMaxTokenConsumptionPerTest, gemini.WithTransport(rec))...)
	require.NoError(t, err, "Failed to create GenAI client")

	return gem, rec.Stop
}

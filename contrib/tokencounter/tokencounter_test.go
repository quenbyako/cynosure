package tokencounter_test

import (
	"maps"
	"net/http"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	. "github.com/quenbyako/cynosure/contrib/tokencounter"
)

const (
	roleUser = "user"
)

func TestTokenCounter(t *testing.T) {
	fsys := NewMemFS()
	tc, err := NewTokenCounter(fsys, &http.Client{})
	require.NoError(t, err)

	msgs := []Message{
		{Role: roleUser, Content: "Hello, this is a local Gemini token count test!"},
	}

	modelTokens := map[string]int{
		"google/gemini-2.5-flash":  11,
		"openai/gpt-4o-mini":       17,
		"openai/gpt-oss-120b":      17,
		"qwen/qwen2.5-7b-instruct": 21,
	}

	for _, modelID := range slices.Sorted(maps.Keys(modelTokens)) {
		t.Run(modelID, func(t *testing.T) {
			count, err := tc.CountTokens(t.Context(), modelID, msgs)
			require.NoError(t, err)
			require.Equal(t, modelTokens[modelID], count)
		})
	}
}

func TestCountEmbeddingTokens(t *testing.T) {
	fsys := NewMemFS()
	tc, err := NewTokenCounter(fsys, &http.Client{})
	require.NoError(t, err)

	text := "Hello, this is a local token count test!"

	// OpenAI (uses cl100k)
	count, err := tc.CountEmbeddingTokens(t.Context(), "openai/text-embedding-3-small", text)
	require.NoError(t, err)
	require.Equal(t, 10, count)

	// Google / Gemini
	count, err = tc.CountEmbeddingTokens(t.Context(), "google/gemini-2.5-flash", text)
	require.NoError(t, err)
	require.Equal(t, 10, count)

	// restriction on unknown models.
	_, err = tc.CountEmbeddingTokens(t.Context(), "unknown/some-model", text)
	require.Error(t, err)
}

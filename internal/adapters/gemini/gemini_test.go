package gemini_test

import (
	"context"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/quenbyako/cynosure/contrib/core-params/ratelimit"
	"github.com/stretchr/testify/require"
	"google.golang.org/genai"

	_ "embed"

	"github.com/quenbyako/cynosure/internal/adapters/gemini/datatransfer"
	chatmodel "github.com/quenbyako/cynosure/internal/domains/cynosure/ports/chatmodel/testsuite"
	embedding "github.com/quenbyako/cynosure/internal/domains/cynosure/ports/embedding/testsuite"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"

	. "github.com/quenbyako/cynosure/internal/adapters/gemini"
)

const (
	maxDuration = time.Duration(math.MaxInt64)
)

var (
	//go:embed .gemini.secret
	apiKey string

	// Setting maximum token usage for tests
	geminiMaxTokenConsumptionPerTest = []NewOption{
		WithEmbeddingLimit(ratelimit.NewPolicy(10000, maxDuration)),
		WithChatInputLimit(ratelimit.NewPolicy(10000, maxDuration)),
	}
)

func TestGeminiChatModel(t *testing.T) {
	gem, err := New(t.Context(), &genai.ClientConfig{
		APIKey: strings.TrimSpace(apiKey),
	},
		append(
			geminiMaxTokenConsumptionPerTest,
			WithLogCallbacks(TestLogCallbacks{t: t}),
		)...,
	)
	require.NoError(t, err, "Failed to create GenAI client")

	chatmodel.RunChatModelTests(gem)(t)
	embedding.RunToolSemanticIndexTests(gem)(t)
}

func TestGeminiWithRotatedKey(t *testing.T) {
	// 1. Setup a transport that injects the real key
	// 2. Setup a config with a dummy key and the custom transport
	// 3. Create the model and run a simple test (Ping)
	transport := &testRotatedKeyTransport{
		base: http.DefaultTransport,
		key:  []byte(strings.TrimSpace(apiKey)),
	}

	cfg := &genai.ClientConfig{
		APIKey: "ROTATED", // GenAI requires non-empty key
		HTTPClient: &http.Client{
			Transport: transport,
		},
	}

	gem, err := New(t.Context(), cfg, append(
		geminiMaxTokenConsumptionPerTest,
		WithLogCallbacks(TestLogCallbacks{t: t}),
	)...)
	require.NoError(t, err, "Failed to create GenAI client with rotated key")

	// If Ping passes, it means the transport successfully injected the real key
	// and replaced "ROTATED".
	chatmodel.RunChatModelTests(gem)(t)
}

type testRotatedKeyTransport struct {
	base http.RoundTripper
	key  []byte
}

func (t *testRotatedKeyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("X-Goog-Api-Key", string(t.key))

	//nolint:wrapcheck // implementing RoundTripper for tests
	return t.base.RoundTrip(req)
}

func TestMessageFromGenAIContent(t *testing.T) {
	for _, tt := range []struct {
		name string
		msgs *genai.GenerateContentResponse
		want []messages.Message
	}{{
		name: "assistant message",
		msgs: &genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{{
				Content: &genai.Content{
					Parts: []*genai.Part{
						genai.NewPartFromText("Hello! I am an AI assistant."),
						genai.NewPartFromText("How can I assist you today?"),
					},
					Role: genai.RoleModel,
				},
			}},
		},
		want: []messages.Message{
			must(messages.NewMessageAssistant("Hello! I am an AI assistant.")),
			must(messages.NewMessageAssistant("How can I assist you today?")),
		},
	}} {
		t.Run(tt.name, func(t *testing.T) {
			agentID := must(ids.RandomAgentID(ids.RandomUserID()))

			want := make([]messages.Message, len(tt.want))
			for i, m := range tt.want {
				msg, ok := m.(messages.MessageAssistant)
				require.True(t, ok, "message is not an assistant message")

				msg, err := messages.NewMessageAssistant(
					msg.Content(),
					messages.WithMessageAssistantAgentID(agentID),
				)
				require.NoError(t, err, "expected no error")

				want[i] = msg
			}

			got, _, _, err := datatransfer.MessageFromGenAIContent(tt.msgs, "", nil, 0, agentID)
			require.NoError(t, err, "expected no error")
			require.Equal(t, want, got, "unexpected message")
		})
	}
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}

	return v
}

type TestLogCallbacks struct {
	t *testing.T
}

var _ LogCallbacks = TestLogCallbacks{}

func (t TestLogCallbacks) GeminiStreamStarted(ctx context.Context, model string, toolCount int) {
	t.t.Helper()
	t.t.Logf("Gemini stream started for model %s with %d tools", model, toolCount)
}

func (t TestLogCallbacks) TokenCountMismatch(
	ctx context.Context, model string, expected, got uint32,
) {
	t.t.Helper()
	t.t.Logf("Token count mismatch for model %s: expected %d, got %d", model, expected, got)
}

package gemini_test

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/quenbyako/cynosure/contrib/core-params/ratelimit"
	"github.com/stretchr/testify/require"
	"google.golang.org/genai"

	"github.com/quenbyako/cynosure/internal/adapters/gemini"
	"github.com/quenbyako/cynosure/internal/adapters/gemini/datatransfer"
	chatmodel "github.com/quenbyako/cynosure/internal/domains/cynosure/ports/chatmodel/testsuite"
	embedding "github.com/quenbyako/cynosure/internal/domains/cynosure/ports/embedding/testsuite"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

// Setting maximum token usage for tests
var geminiMaxTokenConsumptionPerTest = []gemini.NewOption{
	gemini.WithEmbeddingLimit(ratelimit.NewPolicy(10000, maxDuration)),
	gemini.WithChatInputLimit(ratelimit.NewPolicy(10000, maxDuration)),
}

type staticSecretGetter []byte

func (s staticSecretGetter) Get(ctx context.Context) ([]byte, error) { return s, nil }

const (
	maxDuration = time.Duration(math.MaxInt64)
)

func TestAdapter(t *testing.T) {
	gem, stop := vcrModel(t, "testdata/adapter_test")
	t.Cleanup(func() { require.NoError(t, stop()) })

	chatmodel.RunChatModelTests(gem)(t)
	embedding.RunToolSemanticIndexTests(gem)(t)
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

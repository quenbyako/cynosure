package gemini_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/genai"

	_ "embed"

	"github.com/quenbyako/cynosure/internal/adapters/gemini/datatransfer"
	chatmodel "github.com/quenbyako/cynosure/internal/domains/cynosure/ports/chatmodel/testsuite"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/testsuite"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"

	. "github.com/quenbyako/cynosure/internal/adapters/gemini"
)

//go:embed .gemini.secret
var apiKey string

func TestGeminiChatModel(t *testing.T) {
	gem, err := New(t.Context(), &genai.ClientConfig{
		APIKey: apiKey,
	})
	require.NoError(t, err, "Failed to create GenAI client")

	chatmodel.RunChatModelTests(gem)(t)
	testsuite.RunToolSemanticIndexTests(gem)(t)
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

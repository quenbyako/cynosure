package gemini_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/genai"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/testsuite"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/messages"

	. "github.com/quenbyako/cynosure/internal/adapters/gemini"
)

func TestGeminiChatModel(t *testing.T) {
	gem, err := NewGeminiModel(t.Context(), &genai.ClientConfig{
		APIKey: "<REDACTED>",
	})
	require.NoError(t, err, "Failed to create GenAI client")

	testsuite.RunChatModelTests(gem)(t)
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
			got, _, err := MessageFromGenAIContent(tt.msgs, "", 0)
			require.NoError(t, err, "expected no error")
			require.Equal(t, tt.want, got, "unexpected message")
		})
	}
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

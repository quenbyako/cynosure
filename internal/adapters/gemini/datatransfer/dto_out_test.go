package datatransfer_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/genai"

	"github.com/quenbyako/cynosure/internal/adapters/gemini/datatransfer"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

const (
	testLoc        = "loc"
	testGetWeather = "get_weather"
	testCall1      = "call_1"
	testCall2      = "call_2"
)

func TestMessagesToGenAIContent(t *testing.T) {
	for _, tt := range []struct {
		name string
		msgs []messages.Message
		want []*genai.Content
	}{{
		name: "simple user-assistant turn",
		msgs: []messages.Message{
			must(messages.NewMessageUser("Hello")),
			must(messages.NewMessageAssistant("Hi there!")),
		},
		want: []*genai.Content{{
			Role:  genai.RoleUser,
			Parts: []*genai.Part{{Text: "Hello"}},
		}, {
			Role:  genai.RoleModel,
			Parts: []*genai.Part{{Text: "Hi there!"}},
		}},
	}, {
		name: "tool call turn",
		msgs: []messages.Message{
			must(messages.NewMessageUser("What is the weather?")),
			must(messages.NewMessageAssistant("Checking...")),
			must(messages.NewMessageToolRequest(
				map[string]json.RawMessage{testLoc: []byte(`"London"`)},
				testGetWeather, testCall1,
			)),
			must(messages.NewMessageToolResponse(
				[]byte(`{"temp": 20}`),
				testGetWeather, testCall1,
			)),
		},
		want: []*genai.Content{{
			Role:  genai.RoleUser,
			Parts: []*genai.Part{{Text: "What is the weather?"}},
		}, {
			Role: genai.RoleModel,
			Parts: []*genai.Part{
				{Text: "Checking..."},
				{
					FunctionCall: &genai.FunctionCall{
						Name: testGetWeather,
						Args: map[string]any{testLoc: json.RawMessage(`"London"`)},
						ID:   testCall1,
					},
				},
			},
		}, {
			Role: genai.RoleUser,
			Parts: []*genai.Part{{
				FunctionResponse: &genai.FunctionResponse{
					Name: testGetWeather,
					Response: map[string]any{
						"output": json.RawMessage(`{"temp": 20}`),
					},
					ID: testCall1,
				},
			}},
		}},
	}, {
		name: "multiple tool calls in one model turn",
		msgs: []messages.Message{
			must(messages.NewMessageUser("Get weather in London and Paris")),
			must(messages.NewMessageToolRequest(
				map[string]json.RawMessage{testLoc: []byte(`"London"`)},
				testGetWeather, testCall1,
			)),
			must(messages.NewMessageToolRequest(
				map[string]json.RawMessage{testLoc: []byte(`"Paris"`)},
				testGetWeather, testCall2,
			)),
		},
		want: []*genai.Content{{
			Role:  genai.RoleUser,
			Parts: []*genai.Part{{Text: "Get weather in London and Paris"}},
		}, {
			Role: genai.RoleModel,
			Parts: []*genai.Part{{
				FunctionCall: &genai.FunctionCall{
					Name: testGetWeather,
					Args: map[string]any{testLoc: json.RawMessage(`"London"`)},
					ID:   testCall1,
				},
			}, {
				FunctionCall: &genai.FunctionCall{
					Name: testGetWeather,
					Args: map[string]any{testLoc: json.RawMessage(`"Paris"`)},
					ID:   testCall2,
				},
			}},
		}},
	}} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := datatransfer.MessagesToGenAIContent(tt.msgs)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}

	return v
}

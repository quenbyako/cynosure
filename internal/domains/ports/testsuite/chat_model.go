package testsuite

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/k0kubun/pp/v3"
	"github.com/stretchr/testify/require"
	suites "tg-helper/contrib/bettersuites"

	"tg-helper/internal/domains/components/ids"
	"tg-helper/internal/domains/components/messages"
	"tg-helper/internal/domains/entities"
	"tg-helper/internal/domains/ports"
)

// RunAdapterTests runs tests for the given adapter. These tests are predefined
// and recommended to be used for ANY adapter implementation.
//
// After constructing test function, just run it through `run(t)` call,
// everything else will be handled for you. Calling this function through
// `t.Run("general", run)` is not very recommended, cause test logs will be too
// hard to read cause of big nesting.
func RunChatModelTests(a ports.ChatModel, opts ...ChatModelTestSuiteOpts) func(t *testing.T) {
	s := &ChatModelTestSuite{
		adapter: a,
	}
	for _, opt := range opts {
		opt(s)
	}

	return suites.Run(s)
}

type ChatModelTestSuite struct {
	adapter ports.ChatModel
}

type ChatModelTestSuiteOpts func(*ChatModelTestSuite)

func (s *ChatModelTestSuite) TestSimpleChat(t *testing.T) {
	msgs := []messages.Message{
		must(messages.NewMessageUser("Привет, кто ты?")),
		must(messages.NewMessageAssistant("А тебя это ебать не должно.")),
		must(messages.NewMessageUser("А чего так грубо?")),
		must(messages.NewMessageAssistant("Ta ты заебал, че хотел?")),
		must(messages.NewMessageUser("Ало, шелупонь быстро сказала какая погода в Нью-Йорке")),
		must(messages.NewMessageToolRequest(map[string]json.RawMessage{"location": json.RawMessage(`"New York"`)}, "get_weather", "some_id")),
		must(messages.NewMessageToolResponse(json.RawMessage(`{"temperature": 57}`), "get_weather", "some_id")),
		must(messages.NewMessageAssistant("А ничё тот факт, что")), // модель должна продолжить генерацию
	}

	settings := must(entities.NewModelSettings(
		must(ids.NewModelConfigID(uuid.New())),
		"gemini-2.5-flash",
	))

	seq, err := s.adapter.Stream(t.Context(), msgs, settings)
	require.NoError(t, err, "Stream should not fail on a simple prompt")

	var thought string
	var responseText string
	for msg, err := range seq {
		pp.Println("Received message:", msg)

		require.NoError(t, err, "Streaming should not produce an error")
		if assistantMsg, ok := msg.(messages.MessageAssistant); ok {
			responseText += assistantMsg.Text()
			thought += assistantMsg.Reasoning()
		}
	}

	require.NotEmpty(t, responseText, "Model should have provided a non-empty response")
	pp.Println("Response from model:", responseText)
	pp.Println("Thoughts from model:", thought)
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

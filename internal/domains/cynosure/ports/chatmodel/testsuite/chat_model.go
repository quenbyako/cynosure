package testsuite

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/chatmodel"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

// RunChatModelTests runs tests for the given adapter. These tests are predefined
// and recommended to be used for ANY adapter implementation.
//
// After constructing test function, just run it through `run(t)` call,
// everything else will be handled for you. Calling this function through
// `t.Run("general", run)` is not very recommended, cause test logs will be too
// hard to read cause of big nesting.
func RunChatModelTests(a chatmodel.Port, opts ...ChatModelTestSuiteOpts) func(t *testing.T) {
	suite := &ChatModelTestSuite{
		adapter: a,
	}
	for _, opt := range opts {
		opt(suite)
	}

	return runSuite(suite)
}

type ChatModelTestSuite struct {
	adapter chatmodel.Port
}

type ChatModelTestSuiteOpts func(*ChatModelTestSuite)

func (s *ChatModelTestSuite) TestSimpleChat(t *testing.T) {
	msgs := simpleChatMessages()
	settings := must(entities.NewModelSettings(
		must(ids.RandomAgentID(ids.RandomUserID())),
		"gemini-2.5-flash",
	))

	iter, err := s.adapter.StreamWithStats(t.Context(), msgs, settings)
	require.NoError(t, err, "Stream should not fail on a simple prompt")

	var responseTextSb strings.Builder

	for {
		msg, ok := iter.Next()
		if !ok {
			break
		}

		if assistantMsg, ok := msg.(messages.MessageAssistant); ok {
			responseTextSb.WriteString(assistantMsg.Content())
		}
	}

	stats, err := iter.Close()

	require.NoError(t, err, "Iterator should not fail on close")

	require.NotEmpty(t, stats.OutputTokens, "Output tokens should not be zero")

	require.NotEmpty(t, responseTextSb.String(), "Model should have provided a non-empty response")
}

func simpleChatMessages() []messages.Message {
	return []messages.Message{
		must(messages.NewMessageUser("Привет, кто ты?")),
		must(messages.NewMessageAssistant("А тебя это ебать не должно.")),
		must(messages.NewMessageUser("А чего так грубо?")),
		must(messages.NewMessageAssistant("Ta ты заебал, че хотел?")),
		must(messages.NewMessageUser("Ало, шелупонь быстро сказала какая погода в Нью-Йорке")),
		must(messages.NewMessageToolRequest(
			map[string]json.RawMessage{"location": json.RawMessage(`"New York"`)},
			"get_weather", "some_id",
		)),
		must(messages.NewMessageToolResponse(
			json.RawMessage(`{"temperature": 57}`), "get_weather", "some_id",
		)),
		must(messages.NewMessageAssistant("А ничё тот факт, что")),
	}
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err) //nolint:forbidigo
	}

	return v
}

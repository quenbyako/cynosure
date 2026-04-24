package entities_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

func TestThread_TrimmedMessages(t *testing.T) {
	tests := []struct {
		name     string
		messages []messages.Message
		limit    uint
		want     []messages.Message
	}{{
		name:     "Within limit",
		messages: []messages.Message{user(t, "1"), asst(t, "2")},
		limit:    5,
		want:     []messages.Message{user(t, "1"), asst(t, "2")},
	}, {
		name:     "Simple cut",
		messages: []messages.Message{user(t, "1"), asst(t, "2"), user(t, "3"), asst(t, "4")},
		limit:    2,
		want:     []messages.Message{user(t, "3"), asst(t, "4")},
	}, {
		name:     "User cut",
		messages: []messages.Message{user(t, "1"), asst(t, "2"), user(t, "3"), asst(t, "4")},
		limit:    1,
		want:     []messages.Message{asst(t, "4")},
	}, {
		name:     "Orphaned response at start",
		messages: []messages.Message{user(t, "1"), treq(t, "a"), tres(t, "a"), user(t, "2")},
		limit:    2,
		want:     []messages.Message{user(t, "2")},
	}, {
		name: "Orphaned response due to cut",
		messages: []messages.Message{
			user(t, "1"), treq(t, "a"), user(t, "2"), tres(t, "a"), asst(t, "3"),
		},
		limit: 3,
		// window [user("2"), tres("a"), asst("3")] -> tres("a")'s request "a" is cut
		want: []messages.Message{user(t, "2"), asst(t, "3")},
	}, {
		name:     "Full tool chain kept",
		messages: []messages.Message{user(t, "1"), treq(t, "a"), tres(t, "a"), asst(t, "2")},
		limit:    3,
		want:     []messages.Message{treq(t, "a"), tres(t, "a"), asst(t, "2")},
	}, {
		name:     "Partial tool chain cut",
		messages: []messages.Message{treq(t, "a"), treq(t, "b"), tres(t, "a"), tres(t, "b")},
		limit:    3,
		want:     []messages.Message{treq(t, "b"), tres(t, "b")},
	}, {
		name: "Nested tool safety",
		messages: []messages.Message{
			user(t, "1"), treq(t, "a"), asst(t, "2"), tres(t, "a"), user(t, "3"),
		},
		limit: 4,
		want:  []messages.Message{treq(t, "a"), asst(t, "2"), tres(t, "a"), user(t, "3")},
	}, {
		name:     "Start with ToolRequest is safe",
		messages: []messages.Message{user(t, "1"), treq(t, "a"), tres(t, "a")},
		limit:    2,
		want:     []messages.Message{treq(t, "a"), tres(t, "a")},
	}, {
		name:     "Limit 0 returns all",
		messages: []messages.Message{user(t, "1"), asst(t, "2")},
		limit:    0,
		want:     []messages.Message{user(t, "1"), asst(t, "2")},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			threadID, err := ids.RandomThreadID(ids.RandomUserID())
			require.NoError(t, err)

			thread, err := entities.NewThread(threadID, tt.messages)
			require.NoError(t, err)

			got := thread.Messages(tt.limit)
			require.Len(t, got, len(tt.want))

			for i := range tt.want {
				assert.IsType(t, tt.want[i], got[i])

				if u, ok := tt.want[i].(messages.MessageUser); ok {
					gotUser, ok := got[i].(messages.MessageUser)
					require.True(t, ok)
					assert.Equal(t, u.Content(), gotUser.Content())
				}
			}
		})
	}
}

// Setup helpers

func user(t *testing.T, txt string) messages.Message {
	t.Helper()

	msg, err := messages.NewMessageUser(txt)
	require.NoError(t, err)

	return msg
}

func userWithTag(t *testing.T, txt string, tag uint64) messages.MessageUser {
	t.Helper()

	msg, err := messages.NewMessageUser(txt, messages.WithMessageUserMergeTag(tag))
	require.NoError(t, err)

	return msg
}

func asst(t *testing.T, txt string) messages.Message {
	t.Helper()

	agentID, err := ids.RandomAgentID(ids.RandomUserID())
	require.NoError(t, err)
	msg, err := messages.NewMessageAssistant(txt, messages.WithMessageAssistantAgentID(agentID))
	require.NoError(t, err)

	return msg
}

func treq(t *testing.T, id string) messages.Message {
	t.Helper()

	msg, err := messages.NewMessageToolRequest(map[string]json.RawMessage{}, "test_tool", id)
	require.NoError(t, err)

	return msg
}

func tres(t *testing.T, id string) messages.Message {
	t.Helper()

	msg, err := messages.NewMessageToolResponse(json.RawMessage(`{}`), "test_tool", id)
	require.NoError(t, err)

	return msg
}

func TestThread_AddMessage(t *testing.T) {
	threadID, err := ids.RandomThreadID(ids.RandomUserID())
	require.NoError(t, err)

	t.Run("Append message to thread", func(t *testing.T) {
		msg1 := user(t, "first")
		thread, err := entities.NewThread(threadID, []messages.Message{msg1})
		require.NoError(t, err)
		thread.ClearEvents()

		msg2 := user(t, "second")
		err = thread.AddMessage(msg2)
		require.NoError(t, err)

		msgs := thread.Messages(0)
		assert.Len(t, msgs, 2)
		assert.Equal(t, msg1, msgs[0])
		assert.Equal(t, msg2, msgs[1])

		events := thread.PendingEvents()
		require.Len(t, events, 1)
		added, ok := events[0].(entities.ThreadEventMessageAdded)
		require.True(t, ok)
		assert.Equal(t, msg2, added.Message())
	})

	t.Run("Merge message with same non-zero MergeTag", func(t *testing.T) {
		tag := uint64(42)
		msg1 := userWithTag(t, "this i", tag)
		thread, err := entities.NewThread(threadID, []messages.Message{msg1})
		require.NoError(t, err)
		thread.ClearEvents()

		msg2 := userWithTag(t, "s a message", tag)
		err = thread.AddMessage(msg2)
		require.NoError(t, err)

		msgs := thread.Messages(0)
		require.Len(t, msgs, 1)
		msg, ok := msgs[0].(messages.MessageUser)
		require.True(t, ok)
		assert.Equal(t, "this is a message", msg.Content())

		events := thread.PendingEvents()
		require.Len(t, events, 1)
		assert.IsType(t, entities.ThreadEventMessageUpdated{}, events[0])
	})

	t.Run("Do not merge when MergeTag is zero", func(t *testing.T) {
		msg1 := user(t, "first") // MergeTag is 0 by default
		thread, err := entities.NewThread(threadID, []messages.Message{msg1})
		require.NoError(t, err)
		thread.ClearEvents()

		msg2 := user(t, "second")
		err = thread.AddMessage(msg2)
		require.NoError(t, err)

		msgs := thread.Messages(0)
		assert.Len(t, msgs, 2)
		assert.Equal(t, msg2, msgs[1])

		events := thread.PendingEvents()
		require.Len(t, events, 1)
		assert.IsType(t, entities.ThreadEventMessageAdded{}, events[0])
	})

	t.Run("Do not merge when MergeTags are different", func(t *testing.T) {
		msg1 := userWithTag(t, "first", 1)
		thread, err := entities.NewThread(threadID, []messages.Message{msg1})
		require.NoError(t, err)
		thread.ClearEvents()

		msg2 := userWithTag(t, "second", 2)
		err = thread.AddMessage(msg2)
		require.NoError(t, err)

		msgs := thread.Messages(0)
		assert.Len(t, msgs, 2)
		assert.Equal(t, msg2, msgs[1])

		events := thread.PendingEvents()
		require.Len(t, events, 1)
		assert.IsType(t, entities.ThreadEventMessageAdded{}, events[0])
	})
}

func TestThread_Reset(t *testing.T) {
	threadID, err := ids.RandomThreadID(ids.RandomUserID())
	require.NoError(t, err)

	msg1 := user(t, "first")
	thread, err := entities.NewThread(threadID, []messages.Message{msg1})
	require.NoError(t, err)
	thread.ClearEvents()

	t.Run("Undo AddMessage", func(t *testing.T) {
		msg2 := user(t, "second")
		err = thread.AddMessage(msg2)
		require.NoError(t, err)

		assert.Len(t, thread.Messages(0), 2)
		assert.False(t, thread.Synchronized())

		thread.Reset()

		assert.Len(t, thread.Messages(0), 1)
		assert.Equal(t, msg1, thread.Messages(0)[0])
		assert.True(t, thread.Synchronized())
	})

	t.Run("Undo UpdateMessage (Merge)", func(t *testing.T) {
		tag := uint64(777)
		msg := userWithTag(t, "hel", tag)
		thread, err = entities.NewThread(threadID, []messages.Message{msg})
		require.NoError(t, err)
		thread.ClearEvents()

		msg = userWithTag(t, "lo wor", tag)
		err = thread.AddMessage(msg)
		require.NoError(t, err)

		msg = userWithTag(t, "ld!", tag)
		err = thread.AddMessage(msg)
		require.NoError(t, err)

		msgs := thread.Messages(0)
		require.Len(t, msgs, 1)
		msg, ok := msgs[0].(messages.MessageUser)
		require.True(t, ok)
		assert.Equal(t, "hello world!", msg.Content())

		thread.Reset()

		msgs = thread.Messages(0)
		require.Len(t, msgs, 1)
		msg, ok = msgs[0].(messages.MessageUser)
		require.True(t, ok)
		assert.Equal(t, "hel", msg.Content())
		assert.True(t, thread.Synchronized())
	})

	t.Run("Validation error in AddMessage", func(t *testing.T) {
		msg1 := user(t, "first")
		thread, err = entities.NewThread(threadID, []messages.Message{msg1})
		require.NoError(t, err)

		err = thread.AddMessage(invalidMsg{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid")
	})
}

func TestThread_SetAgent(t *testing.T) {
	threadID, err := ids.RandomThreadID(ids.RandomUserID())
	require.NoError(t, err)

	msg1 := user(t, "first")
	thread, err := entities.NewThread(threadID, []messages.Message{msg1})
	require.NoError(t, err)
	thread.ClearEvents()

	agentID, err := ids.RandomAgentID(ids.RandomUserID())
	require.NoError(t, err)

	t.Run("Set new agent", func(t *testing.T) {
		ok := thread.SetAgent(agentID)
		assert.True(t, ok)
		assert.Equal(t, agentID, thread.AgentID())

		events := thread.PendingEvents()
		require.Len(t, events, 1)
		event, ok := events[0].(entities.ThreadEventAgentSet)
		require.True(t, ok)
		assert.Equal(t, agentID, event.AgentID())
	})

	t.Run("Set same agent (no-op)", func(t *testing.T) {
		ok := thread.SetAgent(agentID)
		assert.False(t, ok)
	})

	t.Run("Undo SetAgent", func(t *testing.T) {
		thread.ClearEvents()

		newAgentID, err := ids.RandomAgentID(ids.RandomUserID())
		require.NoError(t, err)
		thread.SetAgent(newAgentID)

		assert.Equal(t, newAgentID, thread.AgentID())
		thread.Reset()
		assert.Equal(t, agentID, thread.AgentID())
	})
}

var _ messages.Message = invalidMsg{}

type invalidMsg struct{ messages.Message }

func (invalidMsg) Valid() bool      { return false }
func (invalidMsg) MergeTag() uint64 { return 0 }

func (invalidMsg) Validate() error {
	return fmt.Errorf("invalid: %w", entities.ErrInternalValidation("invalid"))
}

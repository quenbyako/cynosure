package unit_test

import (
	"context"
	"io"
	"iter"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/a2a"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	a2aadapter "github.com/quenbyako/cynosure/internal/adapters/a2a"
	"github.com/quenbyako/cynosure/internal/domains/gateway/components"
	"github.com/quenbyako/cynosure/internal/domains/gateway/components/ids"
	"github.com/quenbyako/cynosure/internal/domains/gateway/entities"
	"github.com/quenbyako/cynosure/internal/domains/gateway/ports"
	"github.com/quenbyako/cynosure/internal/domains/gateway/usecases"
)

// T062: TestA2AClient_ContextMapping verifies the A2A client properly maps
// channel IDs to context_id and message IDs to message_id
func TestA2AClient_ContextMapping(t *testing.T) {
	tests := []struct {
		name          string
		providerID    string
		channelID     string
		messageID     string
		wantContextID string
		wantMessageID string
	}{
		{
			name:          "telegram provider",
			providerID:    "telegram",
			channelID:     "123456",
			messageID:     "789",
			wantContextID: "telegram/123456",
			wantMessageID: "telegram/123456/789",
		},
		{
			name:          "different provider",
			providerID:    "slack",
			channelID:     "C0123456",
			messageID:     "ts123",
			wantContextID: "slack/C0123456",
			wantMessageID: "slack/C0123456/ts123",
		},
		{
			name:          "numeric IDs",
			providerID:    "telegram",
			channelID:     "987654321",
			messageID:     "123",
			wantContextID: "telegram/987654321",
			wantMessageID: "telegram/987654321/123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mockConn := &mockA2AConn{
				responses: []*a2a.Part{
					{Part: &a2a.Part_Text{Text: "Response"}},
				},
			}
			client := a2aadapter.NewClient(mockConn)

			channelID, err := ids.NewChannelID(tt.providerID, tt.channelID)
			require.NoError(t, err)

			msgID, err := ids.NewMessageID(channelID, tt.messageID)
			require.NoError(t, err)

			text, err := components.NewMessageText("Test message")
			require.NoError(t, err)

			// Act
			_, err = client.Agent().SendMessage(context.Background(), msgID, text)

			// Assert
			require.NoError(t, err)
			require.NotNil(t, mockConn.lastRequest, "expected request to be captured")
			assert.Equal(t, tt.wantContextID, mockConn.lastRequest.Request.ContextId,
				"context_id should match channel ID format")
			assert.Equal(t, tt.wantMessageID, mockConn.lastRequest.Request.MessageId,
				"message_id should match full message ID format")
		})
	}
}

// T063: TestUsecase_ContextIsolation verifies that the usecase properly maintains
// context isolation across different channels
func TestUsecase_ContextIsolation(t *testing.T) {
	// This test ensures that messages from different channels don't interfere
	// with each other's conversation context

	// Arrange - Create mock messenger
	mockMessenger := &mockMessenger{
		sentMessages: make(map[string][]string),
	}

	// Arrange - Create mock A2A agent that tracks contexts
	mockAgent := &mockAgent{
		contexts: make(map[string][]string),
	}

	usecase := usecases.NewUsecase(mockMessenger, mockAgent)

	// Create two different channels
	channel1, err := ids.NewChannelID("telegram", "111111")
	require.NoError(t, err)
	channel2, err := ids.NewChannelID("telegram", "222222")
	require.NoError(t, err)

	// Create messages from each channel
	msg1ID, err := ids.NewMessageID(channel1, "1")
	require.NoError(t, err)
	msg2ID, err := ids.NewMessageID(channel2, "1")
	require.NoError(t, err)

	text1, err := components.NewMessageText("Message to channel 1")
	require.NoError(t, err)
	text2, err := components.NewMessageText("Message to channel 2")
	require.NoError(t, err)

	msg1 := mustNewMessage(msg1ID, text1)
	msg2 := mustNewMessage(msg2ID, text2)

	// Act - Send messages from different channels
	err = usecase.ReceiveNewMessageEvent(context.Background(), msg1)
	require.NoError(t, err)

	err = usecase.ReceiveNewMessageEvent(context.Background(), msg2)
	require.NoError(t, err)

	// Assert - Verify contexts are isolated
	context1 := "telegram/111111"
	context2 := "telegram/222222"

	assert.Contains(t, mockAgent.contexts, context1, "channel 1 should have its own context")
	assert.Contains(t, mockAgent.contexts, context2, "channel 2 should have its own context")

	// Verify each context received its own message
	assert.Len(t, mockAgent.contexts[context1], 1, "channel 1 should have 1 message")
	assert.Len(t, mockAgent.contexts[context2], 1, "channel 2 should have 1 message")

	assert.Equal(t, "Message to channel 1", mockAgent.contexts[context1][0])
	assert.Equal(t, "Message to channel 2", mockAgent.contexts[context2][0])
}

// Mock implementations for testing

type mockMessenger struct {
	ports.Messenger
	sentMessages map[string][]string
}

func (m *mockMessenger) NotifyProcessingStarted(ctx context.Context, channelID ids.ChannelID) error {
	return nil
}

func (m *mockMessenger) SendMessage(ctx context.Context, channelID ids.ChannelID, text components.MessageText) (ids.MessageID, error) {
	m.sentMessages[channelID.String()] = append(m.sentMessages[channelID.String()], text.Text())
	return ids.NewMessageID(channelID, "mock-msg-id")
}

func (m *mockMessenger) UpdateMessage(ctx context.Context, messageID ids.MessageID, text components.MessageText) error {
	return nil
}

type mockAgent struct {
	ports.Agent
	contexts map[string][]string
}

func (m *mockAgent) SendMessage(ctx context.Context, chat ids.MessageID, text components.MessageText) (iter.Seq2[components.MessageText, error], error) {
	contextID := chat.ChannelID().String()
	m.contexts[contextID] = append(m.contexts[contextID], text.Text())

	return func(yield func(components.MessageText, error) bool) {
		resp, _ := components.NewMessageText("Mock response for " + contextID)
		yield(resp, nil)
	}, nil
}

// mockA2AConn for unit tests (reused from contract tests)
type mockA2AConn struct {
	grpc.ClientConnInterface
	lastRequest *a2a.SendMessageRequest
	responses   []*a2a.Part
}

func (m *mockA2AConn) Invoke(ctx context.Context, method string, args, reply any, opts ...grpc.CallOption) error {
	return nil
}

func (m *mockA2AConn) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return &mockClientStream{
		conn:      m,
		responses: m.responses,
	}, nil
}

type mockClientStream struct {
	grpc.ClientStream
	conn      *mockA2AConn
	responses []*a2a.Part
	index     int
	sentMsg   bool
}

func (m *mockClientStream) SendMsg(msg any) error {
	if !m.sentMsg {
		if req, ok := msg.(*a2a.SendMessageRequest); ok {
			m.conn.lastRequest = req
		}
		m.sentMsg = true
	}
	return nil
}

func (m *mockClientStream) RecvMsg(msg any) error {
	if m.index >= len(m.responses) {
		return io.EOF
	}

	if resp, ok := msg.(*a2a.StreamResponse); ok {
		resp.Payload = &a2a.StreamResponse_Msg{
			Msg: &a2a.Message{
				Content: []*a2a.Part{m.responses[m.index]},
			},
		}
		m.index++
		return nil
	}

	return io.EOF
}

func (m *mockClientStream) Header() (metadata.MD, error) { return nil, nil }
func (m *mockClientStream) Trailer() metadata.MD         { return nil }
func (m *mockClientStream) CloseSend() error             { return nil }
func (m *mockClientStream) Context() context.Context     { return context.Background() }

// Helper function
func mustNewMessage(msgID ids.MessageID, text components.MessageText) *entities.Message {
	// Create a test user ID
	userID, err := ids.NewUserID("test", "user123")
	if err != nil {
		panic(err)
	}

	msg, err := entities.NewMessage(msgID, userID, entities.WithText(text))
	if err != nil {
		panic(err)
	}
	return msg
}

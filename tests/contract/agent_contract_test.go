package contract_test

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/a2a"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	a2aadapter "github.com/quenbyako/cynosure/internal/adapters/a2a"
	"github.com/quenbyako/cynosure/internal/domains/gateway/components"
	"github.com/quenbyako/cynosure/internal/domains/gateway/components/ids"
)

// mockA2AConn implements grpc.ClientConnInterface by providing a mock A2A client
type mockA2AConn struct {
	grpc.ClientConnInterface
	lastRequest *a2a.SendMessageRequest
	responses   []*a2a.Part
}

// Invoke handles unary RPCs (not used in our tests)
func (m *mockA2AConn) Invoke(ctx context.Context, method string, args, reply any, opts ...grpc.CallOption) error {
	return nil
}

// NewStream handles streaming RPCs
func (m *mockA2AConn) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return &mockClientStream{
		conn:      m,
		responses: m.responses,
	}, nil
}

// mockClientStream implements grpc.ClientStream for our mock
type mockClientStream struct {
	grpc.ClientStream
	conn      *mockA2AConn
	responses []*a2a.Part
	index     int
	sentMsg   bool
}

func (m *mockClientStream) SendMsg(msg any) error {
	if !m.sentMsg {
		// Capture the request
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

// T060: TestAgentContract_ContextPreservation verifies that the A2A adapter
// correctly uses the channel ID as context_id for conversation context
func TestAgentContract_ContextPreservation(t *testing.T) {
	tests := []struct {
		name          string
		channelID     ids.ChannelID
		messageID     string
		text          string
		wantContextID string
		wantMessageID string
	}{
		{
			name:          "telegram chat preserves context",
			channelID:     mustChannelID("telegram", "123456"),
			messageID:     "789",
			text:          "Hello agent",
			wantContextID: "telegram/123456",
			wantMessageID: "telegram/123456/789",
		},
		{
			name:          "different message same chat same context",
			channelID:     mustChannelID("telegram", "123456"),
			messageID:     "790",
			text:          "Follow up question",
			wantContextID: "telegram/123456",
			wantMessageID: "telegram/123456/790",
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

			msgID, err := ids.NewMessageID(tt.channelID, tt.messageID)
			require.NoError(t, err)

			text, err := components.NewMessageText(tt.text)
			require.NoError(t, err)

			// Act
			_, err = client.Agent().SendMessage(context.Background(), msgID, text)

			// Assert
			require.NoError(t, err)
			assert.NotNil(t, mockConn.lastRequest, "expected request to be captured")
			assert.Equal(t, tt.wantContextID, mockConn.lastRequest.Request.ContextId,
				"context_id should be channel ID for conversation context")
			assert.Equal(t, tt.wantMessageID, mockConn.lastRequest.Request.MessageId,
				"message_id should be full message ID")
		})
	}
}

// T061: TestAgentContract_MultipleChannels verifies that different Telegram chats
// have different conversation contexts (different context_id values)
func TestAgentContract_MultipleChannels(t *testing.T) {
	// Arrange
	mockConn := &mockA2AConn{
		responses: []*a2a.Part{
			{Part: &a2a.Part_Text{Text: "Response"}},
		},
	}
	client := a2aadapter.NewClient(mockConn)

	text, err := components.NewMessageText("Hello")
	require.NoError(t, err)

	// Message from first channel
	channel1 := mustChannelID("telegram", "111111")
	msg1, err := ids.NewMessageID(channel1, "1")
	require.NoError(t, err)

	// Message from second channel
	channel2 := mustChannelID("telegram", "222222")
	msg2, err := ids.NewMessageID(channel2, "1")
	require.NoError(t, err)

	// Act - Send message from first channel
	_, err = client.Agent().SendMessage(context.Background(), msg1, text)
	require.NoError(t, err)
	contextID1 := mockConn.lastRequest.Request.ContextId

	// Act - Send message from second channel
	_, err = client.Agent().SendMessage(context.Background(), msg2, text)
	require.NoError(t, err)
	contextID2 := mockConn.lastRequest.Request.ContextId

	// Assert - Different channels should have different contexts
	assert.NotEqual(t, contextID1, contextID2,
		"different Telegram chats should have different conversation contexts")
	assert.Equal(t, "telegram/111111", contextID1)
	assert.Equal(t, "telegram/222222", contextID2)
}

// Helper functions

func mustChannelID(provider, channel string) ids.ChannelID {
	id, err := ids.NewChannelID(provider, channel)
	if err != nil {
		panic(err)
	}
	return id
}

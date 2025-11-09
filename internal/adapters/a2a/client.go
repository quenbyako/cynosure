package a2a

import (
	"context"
	"errors"
	"fmt"
	"io"
	"iter"

	"google.golang.org/a2a"
	"google.golang.org/grpc"

	"github.com/quenbyako/cynosure/internal/domains/gateway/components"
	"github.com/quenbyako/cynosure/internal/domains/gateway/components/ids"
	"github.com/quenbyako/cynosure/internal/domains/gateway/ports"
)

type Client struct {
	client a2a.A2AServiceClient
}

var _ ports.AgentFactory = (*Client)(nil)

func NewClient(client grpc.ClientConnInterface) *Client {
	if client == nil {
		panic("a2a client connection is required")
	}

	return &Client{
		client: a2a.NewA2AServiceClient(client),
	}
}

func (c *Client) Agent() ports.Agent { return c }

// SendMessage sends a message to the A2A agent and returns a streaming iterator of response chunks.
// Context preservation (T056-T059): The channel ID (chat.ChannelID()) is used as the A2A context_id,
// ensuring that all messages from the same Telegram chat maintain conversation context.
// This enables multi-turn conversations where the agent remembers previous messages.
func (c *Client) SendMessage(ctx context.Context, chat ids.MessageID, text components.MessageText) (iter.Seq2[components.MessageText, error], error) {
	if !chat.Valid() {
		return nil, fmt.Errorf("invalid chat id")
	}
	if !text.Valid() {
		return nil, fmt.Errorf("invalid message text")
	}

	// T056-T058: Use channel ID as context_id for context preservation across messages
	// This ensures that all messages from the same Telegram chat (channelID) maintain
	// the same conversation context in the A2A agent, enabling multi-turn conversations.
	contextID := chat.ChannelID().String()
	messageID := chat.String()

	// T058: Validate context_id consistency per channel
	if contextID == "" {
		return nil, fmt.Errorf("context_id cannot be empty")
	}
	if messageID == "" {
		return nil, fmt.Errorf("message_id cannot be empty")
	}

	response, err := c.client.SendStreamingMessage(ctx, &a2a.SendMessageRequest{
		Request: &a2a.Message{
			MessageId: messageID, // T057: Unique message identifier
			ContextId: contextID, // T056: Channel-based context for multi-turn conversations
			Role:      a2a.Role_ROLE_USER,
			Content: []*a2a.Part{{
				Part: &a2a.Part_Text{Text: text.Text()},
			}},
		},
	})
	// T049: Handle A2A errors gracefully - return error to be handled by usecase
	if err != nil {
		return nil, fmt.Errorf("sending message to a2a: %w", err)
	}

	return func(yield func(components.MessageText, error) bool) {
		for {
			resp, err := response.Recv()
			if errors.Is(err, io.EOF) {
				// Stream has ended
				break
			}

			if err != nil {
				yield(components.MessageText{}, fmt.Errorf("receiving streaming response from a2a: %w", err))
				break
			}

			// T006: Extract text from protobuf Part messages properly
			text, err := extractTextFromA2AMessage(resp.GetMsg())
			if err != nil {
				yield(components.MessageText{}, fmt.Errorf("extracting text from a2a message: %w", err))
				break
			}

			msg, err := components.NewMessageText(text)
			if err != nil {
				yield(components.MessageText{}, fmt.Errorf("invalid message text from a2a: %w", err))
				break
			}

			if !yield(msg, nil) {

				break
			}
		}

	}, nil
}

// T007: Helper function to extract text from A2A protobuf Part messages
func extractTextFromA2AMessage(msg *a2a.Message) (string, error) {
	if msg == nil {
		return "", fmt.Errorf("message is nil")
	}

	var result string
	for _, part := range msg.GetContent() {
		switch p := part.GetPart().(type) {
		case *a2a.Part_Text:
			result += p.Text
		case *a2a.Part_File:
			// Skip file parts for text extraction
			continue
		case *a2a.Part_Data:
			// Skip data parts for text extraction
			continue
		default:
			// Unknown part type, skip
			continue
		}
	}

	if result == "" {
		return "", fmt.Errorf("no text content found in message")
	}

	return result, nil
}

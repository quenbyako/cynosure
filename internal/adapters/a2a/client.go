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

// SendMessage sends a message to the A2A agent and returns a streaming iterator
// of response chunks. The channel ID form [ids.MessageID] is used as the A2A
// context_id, ensuring that all messages from the same Telegram chat maintain
// conversation context. This enables multi-turn conversations where the agent
// remembers previous messages.
func (c *Client) SendMessage(ctx context.Context, chat ids.MessageID, text components.MessageText) (iter.Seq2[components.MessageText, error], error) {
	if !chat.Valid() {
		return nil, fmt.Errorf("invalid chat id")
	}
	if !text.Valid() {
		return nil, fmt.Errorf("invalid message text")
	}

	contextID := chat.ChannelID().String()
	messageID := chat.String()

	response, err := c.client.SendStreamingMessage(ctx, &a2a.SendMessageRequest{
		Request: &a2a.Message{
			MessageId: messageID,
			ContextId: contextID,
			Role:      a2a.Role_ROLE_USER,
			Content: []*a2a.Part{{
				Part: &a2a.Part_Text{Text: text.Text()},
			}},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("sending message to a2a: %w", err)
	}

	return func(yield func(components.MessageText, error) bool) {
		for {
			resp, err := response.Recv()
			if errors.Is(err, io.EOF) {
				break
			}

			if err != nil {
				yield(components.MessageText{}, fmt.Errorf("receiving streaming response from a2a: %w", err))
				break
			}

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

func extractTextFromA2AMessage(msg *a2a.Message) (string, error) {
	if msg == nil {
		return "", fmt.Errorf("message is nil")
	}

	var result string
	for _, part := range msg.GetContent() {
		switch p := part.GetPart().(type) {
		case *a2a.Part_Text:
			result += p.Text
		case *a2a.Part_File, *a2a.Part_Data:
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

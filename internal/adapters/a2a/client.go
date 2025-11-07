package a2a

import (
	"context"
	"errors"
	"fmt"
	"io"
	"iter"

	"google.golang.org/a2a"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

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

func (c *Client) SendMessage(ctx context.Context, chat ids.MessageID, text components.MessageText) (iter.Seq2[components.MessageText, error], error) {
	if !chat.Valid() {
		return nil, fmt.Errorf("invalid chat id")
	}
	if !text.Valid() {
		return nil, fmt.Errorf("invalid message text")
	}

	response, err := c.client.SendStreamingMessage(ctx, &a2a.SendMessageRequest{
		Request: &a2a.Message{
			MessageId: chat.String(),
			ContextId: chat.ChannelID().String(),
			Role:      a2a.Role_ROLE_USER,
			Content: []*a2a.Part{{
				Part: &a2a.Part_Text{Text: text.Text()},
			}},
		},
	})
	if st, ok := status.FromError(err); ok {
		if st.Code() == codes.Unavailable {
			return func(yield func(components.MessageText, error) bool) {
				yield(components.NewMessageText(fmt.Sprintf("A2A service is unavailable - please check your connection configuration: %v", err)))
			}, nil
		}
	} else if err != nil {
		return nil, fmt.Errorf("sending message to a2a: %w", err)
	}

	return func(yield func(components.MessageText, error) bool) {
		// TODO: DO NOT FORGET IN ANY CASE ABOUT STOPPING OF SERVER!!! this is
		// extremely important to cancel this goroutine properly when we are
		// stopping whole application

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

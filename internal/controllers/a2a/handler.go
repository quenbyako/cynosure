// Package a2a implements A2A controller.
package a2a

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/k0kubun/pp/v3"
	"google.golang.org/a2a"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/chat"
)

type Handler struct {
	a2a.UnsafeA2AServiceServer
	srv           *chat.Usecase
	anonymousUser ids.UserID
}

var _ a2a.A2AServiceServer = (*Handler)(nil)

func Register(srv *chat.Usecase, anonUser ids.UserID) func(server grpc.ServiceRegistrar) {
	handler := &Handler{
		UnsafeA2AServiceServer: nil,
		anonymousUser:          anonUser,
		srv:                    srv,
	}

	return func(server grpc.ServiceRegistrar) {
		a2a.RegisterA2AServiceServer(server, handler)
	}
}

// CancelTask implements a2a.A2AServiceServer.
func (h *Handler) CancelTask(context.Context, *a2a.CancelTaskRequest) (*a2a.Task, error) {
	return nil, errors.New("unimplemented")
}

// CreateTaskPushNotificationConfig implements a2a.A2AServiceServer.
func (h *Handler) CreateTaskPushNotificationConfig(
	ctx context.Context, req *a2a.CreateTaskPushNotificationConfigRequest,
) (*a2a.TaskPushNotificationConfig, error) {
	return nil, errors.New("unimplemented")
}

// DeleteTaskPushNotificationConfig implements a2a.A2AServiceServer.
func (h *Handler) DeleteTaskPushNotificationConfig(
	ctx context.Context, req *a2a.DeleteTaskPushNotificationConfigRequest,
) (*emptypb.Empty, error) {
	return nil, errors.New("unimplemented")
}

// GetAgentCard implements a2a.A2AServiceServer.
func (h *Handler) GetAgentCard(context.Context, *a2a.GetAgentCardRequest) (*a2a.AgentCard, error) {
	return nil, errors.New("unimplemented")
}

// GetTask implements a2a.A2AServiceServer.
func (h *Handler) GetTask(context.Context, *a2a.GetTaskRequest) (*a2a.Task, error) {
	return nil, errors.New("unimplemented")
}

// GetTaskPushNotificationConfig implements a2a.A2AServiceServer.
func (h *Handler) GetTaskPushNotificationConfig(
	ctx context.Context, req *a2a.GetTaskPushNotificationConfigRequest,
) (*a2a.TaskPushNotificationConfig, error) {
	return nil, errors.New("unimplemented")
}

// ListTaskPushNotificationConfig implements a2a.A2AServiceServer.
func (h *Handler) ListTaskPushNotificationConfig(
	ctx context.Context, req *a2a.ListTaskPushNotificationConfigRequest,
) (*a2a.ListTaskPushNotificationConfigResponse, error) {
	return nil, errors.New("unimplemented")
}

// SendMessage implements a2a.A2AServiceServer.
func (h *Handler) SendMessage(
	ctx context.Context, req *a2a.SendMessageRequest,
) (*a2a.SendMessageResponse, error) {
	var (
		text     string
		textSb80 strings.Builder
	)

	for _, c := range req.GetRequest().GetContent() {
		textSb80.WriteString(c.GetText())
	}

	text += textSb80.String()

	if len(text) == 0 {
		return nil, errors.New("message content cannot be empty")
	}

	msg, err := messages.NewMessageUser(text)
	if err != nil {
		return nil, fmt.Errorf("creating user message: %w", err)
	}

	threadID, err := ids.NewThreadIDFromString(req.GetRequest().GetContextId())
	if err != nil {
		return nil, fmt.Errorf("parsing thread id: %w", err)
	}

	content, err := h.srv.GenerateResponse(ctx,
		threadID,
		msg,
		chat.WithToolChoice(tools.ToolChoiceForbidden),
	)
	if err != nil {
		return nil, err
	}

	parts := make([]*a2a.Part, 0) // len(content))

	for msg := range content {
		switch msg := msg.(type) {
		case messages.MessageAssistant:
			parts = append(parts, &a2a.Part{
				Part: &a2a.Part_Text{
					Text: msg.Content(),
				},
			})
		case messages.MessageToolRequest:
			argsRaw := make(map[string]any, len(msg.Arguments()))
			for key, value := range msg.Arguments() {
				var x any
				if err := json.Unmarshal(value, &x); err != nil {
					return nil, fmt.Errorf("unmarshalling arg %q: %w", key, err)
				}

				argsRaw[key] = x
			}

			args, err := structpb.NewValue(argsRaw)
			if err != nil {
				return nil, fmt.Errorf("creating args value: %w", err)
			}

			parts = append(parts, &a2a.Part{
				Part: &a2a.Part_Data{
					Data: &a2a.DataPart{
						Data: &structpb.Struct{
							Fields: map[string]*structpb.Value{
								"tool": structpb.NewStringValue(msg.ToolName()),
								"args": args,
							},
						},
					},
				},
			})
		case messages.MessageToolResponse:
			var contentRaw any
			if err := json.Unmarshal(msg.Content(), &contentRaw); err != nil {
				return nil, fmt.Errorf("unmarshalling arg: %w", err)
			}

			content, err := structpb.NewValue(contentRaw)
			if err != nil {
				return nil, fmt.Errorf("creating args value: %w", err)
			}

			parts = append(parts, &a2a.Part{
				Part: &a2a.Part_Data{
					Data: &a2a.DataPart{
						Data: &structpb.Struct{
							Fields: map[string]*structpb.Value{
								"tool":    structpb.NewStringValue(msg.ToolName()),
								"content": content,
							},
						},
					},
				},
			})
		default:
			pp.Println("Unexpected message type:", msg)
		}
	}

	// Simulate sending a message and returning a response.
	return &a2a.SendMessageResponse{
		Payload: &a2a.SendMessageResponse_Msg{
			Msg: &a2a.Message{
				Role:       a2a.Role_ROLE_AGENT,
				Content:    parts,
				MessageId:  "",
				ContextId:  "",
				TaskId:     "",
				Metadata:   nil,
				Extensions: nil,
			},
		},
	}, nil
}

// SendStreamingMessage implements a2a.A2AServiceServer.
func (h *Handler) SendStreamingMessage(
	req *a2a.SendMessageRequest, srv grpc.ServerStreamingServer[a2a.StreamResponse],
) error {
	var (
		text      string
		textSb176 strings.Builder
	)

	for _, c := range req.GetRequest().GetContent() {
		textSb176.WriteString(c.GetText())
	}

	text += textSb176.String()

	if len(text) == 0 {
		return errors.New("message content cannot be empty")
	}

	msg, err := messages.NewMessageUser(text)
	if err != nil {
		return fmt.Errorf("creating user message: %w", err)
	}

	threadID, err := ids.NewThreadIDFromString(req.GetRequest().GetContextId())
	if err != nil {
		return fmt.Errorf("parsing thread id: %w", err)
	}

	content, err := h.srv.GenerateResponse(srv.Context(),
		threadID,
		msg,
		chat.WithToolChoice(tools.ToolChoiceAllowed),
	)
	if err != nil {
		return fmt.Errorf("generating response: %w", err)
	}

	for msg, err := range content {
		if err != nil {
			return fmt.Errorf("generating response: %w", err)
		}

		msg, err := messagesTo(msg)
		if err != nil {
			return fmt.Errorf("converting message: %w", err)
		}

		if err := srv.Send(&a2a.StreamResponse{
			Payload: &a2a.StreamResponse_Msg{Msg: msg},
		}); err != nil {
			return fmt.Errorf("sending message to stream: %w", err)
		}
	}

	return nil
}

// TaskSubscription implements a2a.A2AServiceServer.
func (h *Handler) TaskSubscription(
	req *a2a.TaskSubscriptionRequest, srv grpc.ServerStreamingServer[a2a.StreamResponse],
) error {
	return errors.New("unimplemented")
}

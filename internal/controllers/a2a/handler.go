package a2a

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/k0kubun/pp/v3"
	"google.golang.org/a2a"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/messages"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/tools"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/chat"
)

type Handler struct {
	a2a.UnsafeA2AServiceServer

	anonymousUser ids.UserID

	srv *chat.Service
}

var _ a2a.A2AServiceServer = (*Handler)(nil)

func Register(srv *chat.Service, anonUser ids.UserID) func(server grpc.ServiceRegistrar) {
	handler := &Handler{
		anonymousUser: anonUser,
		srv:           srv,
	}

	return func(server grpc.ServiceRegistrar) {
		a2a.RegisterA2AServiceServer(server, handler)
	}
}

// CancelTask implements a2a.A2AServiceServer.
func (h *Handler) CancelTask(context.Context, *a2a.CancelTaskRequest) (*a2a.Task, error) {
	panic("unimplemented")
}

// CreateTaskPushNotificationConfig implements a2a.A2AServiceServer.
func (h *Handler) CreateTaskPushNotificationConfig(context.Context, *a2a.CreateTaskPushNotificationConfigRequest) (*a2a.TaskPushNotificationConfig, error) {
	panic("unimplemented")
}

// DeleteTaskPushNotificationConfig implements a2a.A2AServiceServer.
func (h *Handler) DeleteTaskPushNotificationConfig(context.Context, *a2a.DeleteTaskPushNotificationConfigRequest) (*emptypb.Empty, error) {
	panic("unimplemented")
}

// GetAgentCard implements a2a.A2AServiceServer.
func (h *Handler) GetAgentCard(context.Context, *a2a.GetAgentCardRequest) (*a2a.AgentCard, error) {
	panic("unimplemented")
}

// GetTask implements a2a.A2AServiceServer.
func (h *Handler) GetTask(context.Context, *a2a.GetTaskRequest) (*a2a.Task, error) {
	panic("unimplemented")
}

// GetTaskPushNotificationConfig implements a2a.A2AServiceServer.
func (h *Handler) GetTaskPushNotificationConfig(context.Context, *a2a.GetTaskPushNotificationConfigRequest) (*a2a.TaskPushNotificationConfig, error) {
	panic("unimplemented")
}

// ListTaskPushNotificationConfig implements a2a.A2AServiceServer.
func (h *Handler) ListTaskPushNotificationConfig(context.Context, *a2a.ListTaskPushNotificationConfigRequest) (*a2a.ListTaskPushNotificationConfigResponse, error) {
	panic("unimplemented")
}

// SendMessage implements a2a.A2AServiceServer.
func (h *Handler) SendMessage(ctx context.Context, req *a2a.SendMessageRequest) (*a2a.SendMessageResponse, error) {
	var text string
	for _, c := range req.Request.GetContent() {
		text += c.GetText()
	}

	if len(text) == 0 {
		return nil, errors.New("message content cannot be empty")
	}

	msg, err := messages.NewMessageUser(text)
	if err != nil {
		return nil, fmt.Errorf("creating user message: %w", err)
	}

	// оказывается, контекст и есть айдишник чата, пользователя мы берем через
	// авторизацию, иначе он анонимен.

	// Пока заглушка
	userID := h.anonymousUser
	threadID := req.GetRequest().GetContextId()

	content, err := h.srv.GenerateResponse(ctx,
		userID,
		threadID,
		msg,
		chat.WithToolChoice(tools.ToolChoiceForbidden),
	)
	if err != nil {
		return nil, err
	}

	parts := make([]*a2a.Part, 0) // len(content))
	for msg := range content {
		switch m := msg.(type) {
		case messages.MessageAssistant:
			parts = append(parts, &a2a.Part{
				Part: &a2a.Part_Text{
					Text: m.Text(),
				},
			})
		case messages.MessageToolRequest:
			argsRaw := make(map[string]any, len(m.Arguments()))
			for k, v := range m.Arguments() {
				var x any
				if err := json.Unmarshal(v, &x); err != nil {
					return nil, fmt.Errorf("unmarshalling arg %q: %w", k, err)
				}
				argsRaw[k] = x
			}

			parts = append(parts, &a2a.Part{
				Part: &a2a.Part_Data{
					Data: &a2a.DataPart{
						Data: &structpb.Struct{
							Fields: map[string]*structpb.Value{
								"tool": structpb.NewStringValue(m.ToolName()),
								"args": must(structpb.NewValue(argsRaw)),
							},
						},
					},
				},
			})
		case messages.MessageToolResponse:
			var content any
			if err := json.Unmarshal(m.Content(), &content); err != nil {
				return nil, fmt.Errorf("unmarshalling arg: %w", err)
			}

			parts = append(parts, &a2a.Part{
				Part: &a2a.Part_Data{
					Data: &a2a.DataPart{
						Data: &structpb.Struct{
							Fields: map[string]*structpb.Value{
								"tool":    structpb.NewStringValue(m.ToolName()),
								"content": must(structpb.NewValue(content)),
							},
						},
					},
				},
			})
		default:
			pp.Println("Unexpected message type:", m)
		}
	}

	// Simulate sending a message and returning a response.
	return &a2a.SendMessageResponse{
		Payload: &a2a.SendMessageResponse_Msg{
			Msg: &a2a.Message{
				Role:    a2a.Role_ROLE_AGENT,
				Content: parts,
			},
		},
	}, nil

}

// SendStreamingMessage implements a2a.A2AServiceServer.
func (h *Handler) SendStreamingMessage(req *a2a.SendMessageRequest, srv grpc.ServerStreamingServer[a2a.StreamResponse]) error {
	var text string
	for _, c := range req.Request.GetContent() {
		text += c.GetText()
	}

	if len(text) == 0 {
		return errors.New("message content cannot be empty")
	}

	msg, err := messages.NewMessageUser(text)
	if err != nil {
		return fmt.Errorf("creating user message: %w", err)
	}

	// оказывается, контекст и есть айдишник чата, пользователя мы берем через
	// авторизацию, иначе он анонимен.

	// Пока заглушка
	userID := uuid.MustParse("ff06b500-0000-0000-0000-000000000001")
	threadID := req.GetRequest().GetContextId()

	content, err := h.srv.GenerateResponse(srv.Context(),
		must(ids.NewUserID(userID)),
		threadID,
		msg,
		chat.WithToolChoice(tools.ToolChoiceAllowed),
	)
	if err != nil {
		return err
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
func (h *Handler) TaskSubscription(*a2a.TaskSubscriptionRequest, grpc.ServerStreamingServer[a2a.StreamResponse]) error {
	panic("unimplemented")
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

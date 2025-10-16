package zep

import (
	"encoding/json"
	"fmt"

	"github.com/getzep/zep-go/v2"
	"github.com/k0kubun/pp/v3"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/messages"
)

func chatHistoryEvents(messages []entities.ChatHistoryEvent) (res []*zep.Message, err error) {
	for _, msg := range messages {
		switch msg := msg.(type) {
		case entities.ChatHistoryEventMessageAdded:
			content, err := messagesToZepContent(msg.Message())
			if err != nil {
				return nil, fmt.Errorf("failed to convert message to Zep format: %w", err)
			}

			res = append(res, content)

		default:
			return nil, fmt.Errorf("unknown message type %T", msg)
		}
	}

	return res, nil
}

func messagesToZepContent(msg messages.Message) (res *zep.Message, err error) {
	switch msg := msg.(type) {
	case messages.MessageAssistant:
		var meta map[string]any
		if reasoning := msg.Reasoning(); reasoning != "" {
			meta = map[string]any{"reasoning": reasoning}
		}

		return &zep.Message{
			RoleType: zep.RoleTypeAssistantRole,
			Content:  msg.Text(),
			Metadata: meta,
		}, nil

	case messages.MessageUser:
		return &zep.Message{
			RoleType: zep.RoleTypeUserRole,
			Content:  msg.Content(),
		}, nil

	case messages.MessageToolRequest:
		content, err := json.Marshal(msg.Arguments())
		if err != nil {
			return nil, fmt.Errorf("failed to marshal tool request arguments: %w", err)
		}

		var meta map[string]any
		if toolCallID := msg.ToolCallID(); toolCallID != "" {
			meta = map[string]any{"tool_call_id": toolCallID}
		}

		return &zep.Message{
			RoleType: zep.RoleTypeToolRole,
			Role:     ptr(msg.ToolName() + ".input"),
			Content:  string(content),
			Metadata: meta,
		}, nil

	case messages.MessageToolResponse:
		pp.Println("encoding tool response!", string(msg.Content()))

		var meta map[string]any
		if toolCallID := msg.ToolCallID(); toolCallID != "" {
			meta = map[string]any{"tool_call_id": toolCallID}
		}

		return &zep.Message{
			RoleType: zep.RoleTypeToolRole,
			Role:     ptr(msg.ToolName() + ".output"),
			Content:  string(msg.Content()),
			Metadata: meta,
		}, nil

	case messages.MessageToolError:
		content, err := json.Marshal(msg.Content())
		if err != nil {
			return nil, fmt.Errorf("failed to marshal tool response content: %w", err)
		}

		var meta map[string]any
		if toolCallID := msg.ToolCallID(); toolCallID != "" {
			meta = map[string]any{"tool_call_id": toolCallID}
		}

		return &zep.Message{
			RoleType: zep.RoleTypeToolRole,
			Role:     ptr(msg.ToolName() + ".error"),
			Content:  string(content),
			Metadata: meta,
		}, nil

	default:
		return nil, fmt.Errorf("unsupported message type %T", msg)
	}
}

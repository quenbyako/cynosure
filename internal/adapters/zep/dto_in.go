package zep

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/getzep/zep-go/v2"

	"tg-helper/internal/domains/components/messages"
)

func MessageFromZepContent(msg *zep.Message) (res messages.Message, err error) {
	switch msg.RoleType {
	case zep.RoleTypeAssistantRole:
		var opts []messages.NewMessageAssistantOpt

		meta := msg.Metadata
		if reasoning, ok := meta["reasoning"].(string); ok {
			opts = append(opts, messages.WithMessageAssistantReasoning(reasoning))
		}

		return messages.NewMessageAssistant(msg.Content, opts...)

	case zep.RoleTypeUserRole:
		return messages.NewMessageUser(msg.Content)

	case zep.RoleTypeToolRole:
		toolName := ""
		if msg.Role != nil {
			toolName = *msg.Role
		}

		meta := msg.Metadata

		toolCallID, ok := meta["tool_call_id"].(string)
		if !ok {
			return nil, fmt.Errorf("tool call ID is missing in tool message metadata")
		}

		if strings.HasSuffix(toolName, ".input") {
			toolName = strings.TrimSuffix(toolName, ".input")

			args := make(map[string]json.RawMessage)
			if err := json.Unmarshal([]byte(msg.Content), &args); err != nil {
				return nil, fmt.Errorf("failed to unmarshal tool arguments: %w", err)
			}

			return messages.NewMessageToolRequest(args, toolName, toolCallID)

		} else if strings.HasSuffix(toolName, ".output") {
			toolName = strings.TrimSuffix(toolName, ".output")
		}

		return messages.NewMessageToolResponse(json.RawMessage(msg.Content), toolName, toolCallID)

	default:
		return nil, fmt.Errorf("unknown role type %q", msg.RoleType)
	}
}

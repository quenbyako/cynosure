package chatmodel

import (
	"encoding/json"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.39.0"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/chatmodel/genai"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

func marshalMessagesToGenai(systemMessage string, msgs []messages.Message) attribute.KeyValue {
	converted, err := otelConcatMsgsGenAI(systemMessage, msgs)
	if err != nil {
		return semconv.GenAIInputMessagesKey.String(fmt.Sprintf("SERIALIZING ERROR: %v", err))
	}

	msgsJSON, err := json.Marshal(converted)
	if err != nil {
		return semconv.GenAIInputMessagesKey.String(fmt.Sprintf("SERIALIZING ERROR: %v", err))
	}

	return semconv.GenAIInputMessagesKey.String(string(msgsJSON))
}

func marshalResponseToGenai(out []genai.ChatMessage, reason genai.FinishReason) attribute.KeyValue {
	if len(out) > 0 {
		out[len(out)-1].FinishReason = reason
	}

	msgsJSON, err := json.Marshal(out)
	if err != nil {
		return semconv.GenAIOutputMessagesKey.String(fmt.Sprintf("SERIALIZING ERROR: %v", err))
	}

	return semconv.GenAIOutputMessagesKey.String(string(msgsJSON))
}

func otelConcatMsgsGenAI(systemMsg string, msgs []messages.Message) ([]genai.ChatMessage, error) {
	res := make([]genai.ChatMessage, 0, len(msgs)+1)
	if systemMsg != "" {
		res = append(res, genai.ChatMessage{
			Role: genai.RoleSystem,
			Name: nil,
			Parts: []genai.MessagePart{genai.TextPart{
				Type:    "text",
				Content: systemMsg,
			}},
			FinishReason: "",
		})
	}

	var (
		current *genai.ChatMessage
		err     error
	)

	for i, m := range msgs {
		current, err = concatenateMessage(&res, current, m)
		if err != nil {
			return nil, fmt.Errorf("converting message %d: %w", i, err)
		}
	}

	if current != nil {
		res = append(res, *current)
	}

	return res, nil
}

func concatenateMessage(
	res *[]genai.ChatMessage, current *genai.ChatMessage, msg messages.Message,
) (*genai.ChatMessage, error) {
	switch msg := msg.(type) {
	case messages.MessageAssistant:
		return concatenateMessageAssistant(res, current, msg)
	case messages.MessageToolRequest:
		return concatenateMessageToolRequest(res, current, msg)
	case messages.MessageToolResponse:
		return concatenateMessageToolResponse(res, current, msg)
	case messages.MessageUser:
		return concatenateMessageUser(res, current, msg)
	}

	return current, nil
}

func concatenateMessageAssistant(
	res *[]genai.ChatMessage, current *genai.ChatMessage, msg messages.MessageAssistant,
) (*genai.ChatMessage, error) {
	current = ensureRole(res, current, genai.RoleAssistant)
	current.Parts = append(current.Parts, genai.TextPart{
		Type:    "text",
		Content: msg.Content(),
	})

	return current, nil
}

func concatenateMessageToolRequest(
	res *[]genai.ChatMessage, current *genai.ChatMessage, msg messages.MessageToolRequest,
) (*genai.ChatMessage, error) {
	args, err := json.Marshal(msg.Arguments())
	if err != nil {
		return nil, fmt.Errorf("marshalling arguments for tool request: %w", err)
	}

	current = ensureRole(res, current, genai.RoleAssistant)
	current.Parts = append(current.Parts, genai.ToolCallRequestPart{
		Type:      "tool_call",
		ID:        ptr(msg.ToolCallID()),
		Name:      msg.ToolName(),
		Arguments: args,
	})

	return current, nil
}

func concatenateMessageToolResponse(
	res *[]genai.ChatMessage, current *genai.ChatMessage, msg messages.MessageToolResponse,
) (*genai.ChatMessage, error) {
	current = ensureRole(res, current, genai.RoleTool)
	current.Parts = append(current.Parts, genai.ToolCallResponsePart{
		Type:     "tool_call_response",
		ID:       ptr(msg.ToolCallID()),
		Response: msg.Content(),
	})

	return current, nil
}

func concatenateMessageUser(
	res *[]genai.ChatMessage, current *genai.ChatMessage, msg messages.MessageUser,
) (*genai.ChatMessage, error) {
	current = ensureRole(res, current, genai.RoleUser)
	current.Parts = append(current.Parts, genai.TextPart{
		Type:    "text",
		Content: msg.Content(),
	})

	return current, nil
}

// ensureRole either flushes the current message to the result slice if its role
// differs from the new one, or creates a new message if current is nil.
//
// This function helps to concatenate multiple messages from the same role into
// a single one.
func ensureRole(
	res *[]genai.ChatMessage, current *genai.ChatMessage, role genai.Role,
) *genai.ChatMessage {
	if current != nil && current.Role != role {
		*res = append(*res, *current)
		current = nil
	}

	if current == nil {
		current = &genai.ChatMessage{
			Role:         role,
			Name:         nil,
			Parts:        nil,
			FinishReason: "",
		}
	}

	return current
}

func ptr[T any](v T) *T { return &v }

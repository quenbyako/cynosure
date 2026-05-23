package datatransfer

import (
	"encoding/json"
	"fmt"

	"github.com/OpenRouterTeam/go-sdk/models/components"
	"github.com/OpenRouterTeam/go-sdk/optionalnullable"
	"github.com/quenbyako/cynosure/contrib/tokencounter"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
)

// ConvertToolChoice maps domain ToolChoice to components.ChatToolChoice.
func ConvertToolChoice(choice tools.ToolChoice) (components.ChatToolChoice, error) {
	switch choice {
	case tools.ToolChoiceAllowed:
		return components.CreateChatToolChoiceChatToolChoiceAuto(
			components.ChatToolChoiceAutoAuto,
		), nil
	case tools.ToolChoiceForced:
		return components.CreateChatToolChoiceChatToolChoiceRequired(
			components.ChatToolChoiceRequiredRequired,
		), nil
	case tools.ToolChoiceForbidden:
		return components.CreateChatToolChoiceChatToolChoiceNone(
			components.ChatToolChoiceNoneNone,
		), nil
	default:
		return components.ChatToolChoice{}, fmt.Errorf("%w: %v", ErrUnknownToolChoice, choice)
	}
}

// ConvertMessages maps domain Messages to a slice of components.ChatMessages.
func ConvertMessages(systemMsg string, input []messages.Message) []components.ChatMessages {
	var openAIMsgs []components.ChatMessages

	if systemMsg != "" {
		msg := components.CreateChatMessagesSystem(components.ChatSystemMessage{
			Role:    components.ChatSystemMessageRoleSystem,
			Content: components.CreateChatSystemMessageContentStr(systemMsg),
			Name:    nil,
		})
		openAIMsgs = append(openAIMsgs, msg)
	}

	for _, m := range input {
		openAIMsgs = AppendConvertedMessage(openAIMsgs, m)
	}

	return openAIMsgs
}

// AppendConvertedMessage appends a single converted message to components.ChatMessages slice.
func AppendConvertedMessage(
	msgs []components.ChatMessages,
	m messages.Message,
) []components.ChatMessages {
	switch msg := m.(type) {
	case messages.MessageUser:
		return append(msgs, components.CreateChatMessagesUser(components.ChatUserMessage{
			Role:    components.ChatUserMessageRoleUser,
			Content: components.CreateChatUserMessageContentStr(msg.Content()),
			Name:    nil,
		}))
	case messages.MessageAssistant:
		return appendAssistantMessage(msgs, msg)
	case messages.MessageToolRequest:
		return AppendToolRequest(msgs, msg)
	case messages.MessageToolResponse:
		return append(msgs, components.CreateChatMessagesTool(components.ChatToolMessage{
			Role:       components.ChatToolMessageRoleTool,
			ToolCallID: msg.ToolCallID(),
			Content:    components.CreateChatToolMessageContentStr(string(msg.Content())),
		}))
	case messages.MessageToolError:
		return append(msgs, components.CreateChatMessagesTool(components.ChatToolMessage{
			Role:       components.ChatToolMessageRoleTool,
			ToolCallID: msg.ToolCallID(),
			Content:    components.CreateChatToolMessageContentStr(string(msg.Content())),
		}))
	}

	return msgs
}

func appendAssistantMessage(
	msgs []components.ChatMessages,
	msg messages.MessageAssistant,
) []components.ChatMessages {
	content := components.CreateChatAssistantMessageContentStr(msg.Content())

	return append(msgs, components.CreateChatMessagesAssistant(components.ChatAssistantMessage{
		Role:             components.ChatAssistantMessageRoleAssistant,
		Content:          optionalnullable.From(&content),
		Audio:            nil,
		Images:           nil,
		Name:             nil,
		Reasoning:        nil,
		ReasoningDetails: nil,
		Refusal:          nil,
		ToolCalls:        nil,
	}))
}

func marshalToolArguments(args map[string]json.RawMessage) string {
	argsMap := make(map[string]any)
	for k, v := range args {
		argsMap[k] = v
	}

	argsData, err := json.Marshal(argsMap)
	if err != nil {
		panic(err) //nolint:forbidigo // unreachable
	}

	return string(argsData)
}

// AppendToolRequest appends a message containing tool requests to components.ChatMessages slice.
func AppendToolRequest(
	msgs []components.ChatMessages,
	msg messages.MessageToolRequest,
) []components.ChatMessages {
	var tc components.ChatToolCall

	tc.ID = msg.ToolCallID()
	tc.Type = components.ChatToolCallTypeFunction
	tc.Function.Name = msg.ToolName()
	tc.Function.Arguments = marshalToolArguments(msg.Arguments())

	lastIdx := len(msgs) - 1
	if lastIdx >= 0 && msgs[lastIdx].Type == components.ChatMessagesTypeAssistant {
		msgs[lastIdx].ChatAssistantMessage.ToolCalls = append(
			msgs[lastIdx].ChatAssistantMessage.ToolCalls, tc,
		)

		return msgs
	}

	return append(msgs, components.CreateChatMessagesAssistant(components.ChatAssistantMessage{
		Role:             components.ChatAssistantMessageRoleAssistant,
		Content:          nil,
		Audio:            nil,
		Images:           nil,
		Name:             nil,
		Reasoning:        nil,
		ReasoningDetails: nil,
		Refusal:          nil,
		ToolCalls:        []components.ChatToolCall{tc},
	}))
}

// ConvertMessagesForTokenCounting converts messages to the tokencounter.Message
// format.
func ConvertMessagesForTokenCounting(
	systemMessage string,
	msgs []messages.Message,
) []tokencounter.Message {
	var result []tokencounter.Message
	if systemMessage != "" {
		result = append(result, tokencounter.Message{
			Role:    "system",
			Content: systemMessage,
		})
	}

	for _, m := range msgs {
		result = appendTokenMessage(result, m)
	}

	return result
}

func formatToolRequestTokenContent(msg messages.MessageToolRequest) string {
	argsMap := make(map[string]any)
	for k, v := range msg.Arguments() {
		argsMap[k] = v
	}

	argsData, err := json.Marshal(argsMap)
	if err != nil {
		panic(err) //nolint:forbidigo // unreachable
	}

	return msg.ToolName() + " " + string(argsData)
}

func appendTokenMessage(result []tokencounter.Message, m messages.Message) []tokencounter.Message {
	switch msg := m.(type) {
	case messages.MessageUser:
		return append(result, tokencounter.Message{
			Role:    "user",
			Content: msg.Content(),
		})
	case messages.MessageAssistant:
		return append(result, tokencounter.Message{
			Role:    "assistant",
			Content: msg.Content(),
		})
	case messages.MessageToolRequest:
		return append(result, tokencounter.Message{
			Role:    "assistant",
			Content: formatToolRequestTokenContent(msg),
		})
	case messages.MessageToolResponse:
		return append(result, tokencounter.Message{
			Role:    "tool",
			Content: string(msg.Content()),
		})
	case messages.MessageToolError:
		return append(result, tokencounter.Message{
			Role:    "tool",
			Content: string(msg.Content()),
		})
	}

	return result
}

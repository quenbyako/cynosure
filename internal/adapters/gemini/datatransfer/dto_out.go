package datatransfer

import (
	"encoding/json"
	"fmt"

	"google.golang.org/genai"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
)

// MessagesToGenAIContent converts internal messages to Gemini content format.
func MessagesToGenAIContent(msgs []messages.Message) ([]*genai.Content, error) {
	res := make([]*genai.Content, 0, len(msgs))

	for _, msg := range msgs {
		switch typedMsg := msg.(type) {
		case messages.MessageUser:
			res = append(res, convertUserMsg(typedMsg))
		case messages.MessageAssistant:
			res = append(res, convertAssistantMsg(typedMsg))
		case messages.MessageToolRequest:
			content, err := convertToolReqMsg(typedMsg, res)
			if err != nil {
				return nil, err
			}

			if content != nil {
				res = append(res, content)
			}
		case messages.MessageToolResponse:
			res = append(res, convertToolRespMsg(typedMsg, res))
		case messages.MessageToolError:
			res = append(res, convertToolErrorMsg(typedMsg, res))
		default:
			return nil, fmt.Errorf("%w: %T", ErrUnsupportedMsgType, typedMsg)
		}
	}

	return res, nil
}

func convertUserMsg(msg messages.MessageUser) *genai.Content {
	return &genai.Content{
		Role: genai.RoleUser,
		Parts: []*genai.Part{
			{
				Text:                msg.Content(),
				MediaResolution:     nil,
				CodeExecutionResult: nil,
				ExecutableCode:      nil,
				FileData:            nil,
				FunctionCall:        nil,
				FunctionResponse:    nil,
				InlineData:          nil,
				Thought:             false,
				ThoughtSignature:    nil,
				VideoMetadata:       nil,
			},
		},
	}
}

func convertAssistantMsg(msg messages.MessageAssistant) *genai.Content {
	var parts []*genai.Part

	if reasoning := msg.Reasoning(); reasoning != "" {
		parts = append(parts, assistantReasoningPart(reasoning))
	}

	parts = append(parts, assistantContentPart(msg.Content()))

	return &genai.Content{
		Role:  genai.RoleModel,
		Parts: parts,
	}
}

func assistantReasoningPart(reasoning string) *genai.Part {
	return &genai.Part{
		Thought:             true,
		Text:                reasoning,
		MediaResolution:     nil,
		CodeExecutionResult: nil,
		ExecutableCode:      nil,
		FileData:            nil,
		FunctionCall:        nil,
		FunctionResponse:    nil,
		InlineData:          nil,
		ThoughtSignature:    nil,
		VideoMetadata:       nil,
		ToolCall:            nil,
		ToolResponse:        nil,
		PartMetadata:        nil,
	}
}

func assistantContentPart(text string) *genai.Part {
	return &genai.Part{
		Text:                text,
		MediaResolution:     nil,
		CodeExecutionResult: nil,
		ExecutableCode:      nil,
		FileData:            nil,
		FunctionCall:        nil,
		FunctionResponse:    nil,
		InlineData:          nil,
		Thought:             false,
		ThoughtSignature:    nil,
		VideoMetadata:       nil,
		ToolCall:            nil,
		ToolResponse:        nil,
		PartMetadata:        nil,
	}
}

func convertToolReqMsg(
	msg messages.MessageToolRequest,
	res []*genai.Content,
) (*genai.Content, error) {
	if len(res) == 0 {
		return nil, ErrOrphanedToolResp
	}

	last := res[len(res)-1]
	if last.Role != genai.RoleModel {
		return &genai.Content{
			Role:  genai.RoleModel,
			Parts: nil,
		}, nil
	}

	appendToolCall(last, msg)

	return nil, ErrNilMsg // return sentinel error instead of nil, nil
}

func appendToolCall(last *genai.Content, msg messages.MessageToolRequest) {
	args := make(map[string]any)
	for key, val := range msg.Arguments() {
		args[key] = val
	}

	part := genai.NewPartFromFunctionCall(msg.ToolName(), args)

	if protocolMeta := msg.ProtocolMetadata(); protocolMeta != nil {
		var content struct {
			Sig []byte `json:"gemini_thought_signature"`
		}

		if err := json.Unmarshal(protocolMeta, &content); err == nil {
			part.ThoughtSignature = content.Sig
		}
	}

	last.Parts = append(last.Parts, part)
}

func convertToolRespMsg(msg messages.MessageToolResponse, res []*genai.Content) *genai.Content {
	last := ensureUserContent(res)
	part := genai.NewPartFromFunctionResponse(msg.ToolName(), map[string]any{
		"output": msg.Content(),
	})

	last.Parts = append(last.Parts, part)

	return last
}

func convertToolErrorMsg(msg messages.MessageToolError, res []*genai.Content) *genai.Content {
	last := ensureUserContent(res)
	part := genai.NewPartFromFunctionResponse(msg.ToolName(), map[string]any{
		"error": msg.Content(),
	})

	last.Parts = append(last.Parts, part)

	return last
}

func ensureUserContent(res []*genai.Content) *genai.Content {
	if len(res) > 0 && res[len(res)-1].Role == genai.RoleUser {
		return res[len(res)-1]
	}

	return &genai.Content{
		Role:  genai.RoleUser,
		Parts: nil,
	}
}

// ToolInfoToGenAI converts tool definitions to Gemini tool format.
func ToolInfoToGenAI(rawTools []tools.RawTool) []*genai.Tool {
	decls := make([]*genai.FunctionDeclaration, len(rawTools))

	for i, t := range rawTools {
		decls[i] = &genai.FunctionDeclaration{
			Name:                 t.Name(),
			Description:          t.Desc(),
			ParametersJsonSchema: t.ConvertedSchema(),
			ResponseJsonSchema:   t.Response(),
			Parameters:           nil,
			Response:             nil,
			Behavior:             "",
		}
	}

	return []*genai.Tool{{
		FunctionDeclarations: decls,
	}}
}

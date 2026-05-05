package datatransfer

import (
	"encoding/json"
	"fmt"

	"github.com/quenbyako/ext/set"
	"google.golang.org/genai"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
)

// MessagesToGenAIContent converts internal messages to Gemini content format.
func MessagesToGenAIContent(msgs []messages.Message) ([]*genai.Content, error) {
	var (
		res       = make([]*genai.Content, 0, len(msgs))
		toolCalls = set.New[string]()
		current   genai.Content
		err       error
	)

	for _, msg := range msgs {
		switch msg := msg.(type) {
		case messages.MessageUser:
			res = convertUserMsg(res, &current, msg)
		case messages.MessageAssistant:
			res = convertAssistantMsg(res, &current, msg)
		case messages.MessageToolRequest:
			res, err = convertToolReqMsg(res, &current, toolCalls, msg)
		case messages.MessageToolResponse:
			res, err = convertToolRespMsg(res, &current, toolCalls, msg)
		case messages.MessageToolError:
			res, err = convertToolErrorMsg(res, &current, toolCalls, msg)
		default:
			err = fmt.Errorf("%w: %T", ErrUnsupportedMsgType, msg)
		}

		if err != nil {
			return nil, err
		}
	}

	return pushContent(res, &current), nil
}

func convertUserMsg(res []*genai.Content, current *genai.Content, msg messages.MessageUser) []*genai.Content {
	if current.Role == genai.RoleModel {
		res = pushContent(res, current)
	}

	current.Role = genai.RoleUser

	current.Parts = append(current.Parts, genai.NewPartFromText(msg.Content()))

	return res
}

func convertAssistantMsg(res []*genai.Content, current *genai.Content, msg messages.MessageAssistant) []*genai.Content {
	if current.Role == genai.RoleUser {
		res = pushContent(res, current)
	}

	current.Role = genai.RoleModel

	if reasoning := msg.Reasoning(); reasoning != "" {
		current.Parts = append(current.Parts, assistantReasoningPart(reasoning))
	}

	current.Parts = append(current.Parts, assistantContentPart(msg.Content()))

	return res
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

func convertToolReqMsg(res []*genai.Content, current *genai.Content, toolCalls set.Set[string], msg messages.MessageToolRequest) ([]*genai.Content, error) {
	if toolCalls.Has(msg.ToolCallID()) {
		return nil, fmt.Errorf("%w: %s", ErrDuplicatedToolCallID, msg.ToolCallID())
	}

	if current.Role == genai.RoleUser {
		res = pushContent(res, current)
	}

	current.Role = genai.RoleModel

	toolCalls.Add(msg.ToolCallID())
	current.Parts = append(current.Parts, buildToolCallPart(msg))

	return res, nil
}

func buildToolCallPart(msg messages.MessageToolRequest) *genai.Part {
	args := make(map[string]any)
	for key, val := range msg.Arguments() {
		args[key] = val
	}

	part := genai.NewPartFromFunctionCall(msg.ToolName(), args)
	if part.FunctionCall != nil {
		part.FunctionCall.ID = msg.ToolCallID()
	}

	if protocolMeta := msg.ProtocolMetadata(); protocolMeta != nil {
		var content struct {
			Sig []byte `json:"gemini_thought_signature"`
		}

		if err := json.Unmarshal(protocolMeta, &content); err == nil {
			part.ThoughtSignature = content.Sig
		}
	}

	return part
}

func convertToolRespMsg(res []*genai.Content, current *genai.Content, toolCalls set.Set[string], msg messages.MessageToolResponse) ([]*genai.Content, error) {
	if !toolCalls.Has(msg.ToolCallID()) {
		return nil, ErrOrphanedToolResp
	}

	if current.Role == genai.RoleModel {
		res = pushContent(res, current)
	}

	current.Role = genai.RoleUser

	part := genai.NewPartFromFunctionResponse(msg.ToolName(), map[string]any{
		"output": msg.Content(),
	})
	if part.FunctionResponse != nil {
		part.FunctionResponse.ID = msg.ToolCallID()
	}

	current.Parts = append(current.Parts, part)

	return res, nil
}

func convertToolErrorMsg(res []*genai.Content, current *genai.Content, toolCalls set.Set[string], msg messages.MessageToolError) ([]*genai.Content, error) {
	if !toolCalls.Has(msg.ToolCallID()) {
		return nil, ErrOrphanedToolResp
	}

	if current.Role == genai.RoleModel {
		res = pushContent(res, current)
	}

	current.Role = genai.RoleUser

	part := genai.NewPartFromFunctionResponse(msg.ToolName(), map[string]any{
		"error": msg.Content(),
	})
	if part.FunctionResponse != nil {
		part.FunctionResponse.ID = msg.ToolCallID()
	}

	current.Parts = append(current.Parts, part)

	return res, nil
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

func pushContent(res []*genai.Content, current *genai.Content) []*genai.Content {
	if current == nil {
		return res
	}

	cloned := &genai.Content{
		// pointer to this slice will be deleted, so just using same ref
		Parts: current.Parts,
		Role:  current.Role,
	}
	*current = genai.Content{}

	return append(res, cloned)
}

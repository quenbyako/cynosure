package datatransfer

import (
	"encoding/json"
	"fmt"

	"google.golang.org/genai"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
)

func MessagesToGenAIContent(msg []messages.Message) (res []*genai.Content, err error) {
	for i, m := range msg {
		switch m := m.(type) {
		case messages.MessageUser:
			res = append(res, &genai.Content{
				Role: genai.RoleUser,
				Parts: []*genai.Part{
					genai.NewPartFromText(m.Content()),
					// TODO: Handle attachments if needed
				},
			})

		case messages.MessageAssistant:
			var parts []*genai.Part

			if m.Reasoning() != "" {
				parts = append(parts, &genai.Part{
					Thought:             true,
					Text:                m.Reasoning(),
					MediaResolution:     nil,
					CodeExecutionResult: nil,
					ExecutableCode:      nil,
					FileData:            nil,
					FunctionCall:        nil,
					FunctionResponse:    nil,
					InlineData:          nil,
					ThoughtSignature:    nil,
					VideoMetadata:       nil,
				})
			}

			parts = append(parts, genai.NewPartFromText(m.Content()))

			res = append(res, &genai.Content{
				Role:  genai.RoleModel,
				Parts: parts,
			})

		case messages.MessageToolRequest:
			if len(res) == 0 {
				return nil, fmt.Errorf("tool response message %v is orphaned, no assistant message found", i)
			}

			lastContent := res[len(res)-1]
			if lastContent.Role != genai.RoleModel {
				// it's okay, if model has not anything to say
				// we just add empty content
				res = append(res, &genai.Content{
					Role:  genai.RoleModel,
					Parts: nil,
				})
				lastContent = res[len(res)-1]
			}

			args := make(map[string]any)
			for k, v := range m.Arguments() {
				args[k] = v
			}

			part := genai.NewPartFromFunctionCall(m.ToolName(), args)
			if sig := m.ProtocolMetadata(); sig != nil {
				var thoughtSig struct {
					Sig []byte `json:"gemini_thought_signature"`
				}
				if err := json.Unmarshal(sig, &thoughtSig); err == nil {
					part.ThoughtSignature = thoughtSig.Sig
				}
			}

			lastContent.Parts = append(lastContent.Parts, part)

		case messages.MessageToolResponse:
			var lastContent *genai.Content
			if len(res) > 0 {
				lastContent = res[len(res)-1]
			}

			if lastContent == nil || lastContent.Role != genai.RoleUser {
				lastContent = &genai.Content{
					Role:  genai.RoleUser,
					Parts: nil,
				}
				res = append(res, lastContent)
			}

			lastContent.Parts = append(lastContent.Parts, genai.NewPartFromFunctionResponse(m.ToolName(), map[string]any{
				"output": m.Content(),
			}))

		case messages.MessageToolError:
			var lastContent *genai.Content
			if len(res) > 0 {
				lastContent = res[len(res)-1]
			}

			if lastContent == nil || lastContent.Role != genai.RoleUser {
				lastContent = &genai.Content{
					Role:  genai.RoleUser,
					Parts: nil,
				}
				res = append(res, lastContent)
			}

			lastContent.Parts = append(lastContent.Parts, genai.NewPartFromFunctionResponse(m.ToolName(), map[string]any{
				"error": m.Content(),
			}))

		default:
			return nil, fmt.Errorf("unsupported message type: %T", m)
		}
	}

	return res, nil
}

func ToolInfoToGenAI(tools []tools.RawTool) (res []*genai.Tool) {
	decls := make([]*genai.FunctionDeclaration, len(tools))
	for i, tool := range tools {
		decls[i] = &genai.FunctionDeclaration{
			Name:                 tool.Name(),
			Description:          tool.Desc(),
			ParametersJsonSchema: tool.ConvertedSchema(),
			ResponseJsonSchema:   tool.Response(),
			Parameters:           nil,
			Response:             nil,
			Behavior:             "",
		}
	}

	return []*genai.Tool{{
		FunctionDeclarations: decls,
	}}
}

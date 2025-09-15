package gemini

import (
	"fmt"

	"google.golang.org/genai"

	"tg-helper/internal/domains/components/messages"
	"tg-helper/internal/domains/components/tools"
)

func messagesToGenAIContent(msg []messages.Message) (res []*genai.Content, err error) {
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
				parts = append(parts, &genai.Part{Thought: true, Text: m.Reasoning()})
			}

			parts = append(parts, genai.NewPartFromText(m.Text()))

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
					Role: genai.RoleModel,
				})
				lastContent = res[len(res)-1]
			}

			args := make(map[string]any)
			for k, v := range m.Arguments() {
				args[k] = v
			}

			lastContent.Parts = append(lastContent.Parts, genai.NewPartFromFunctionCall(m.ToolName(), args))

		case messages.MessageToolResponse:
			if len(res) == 0 {
				return nil, fmt.Errorf("tool response message %v is orphaned, no assistant message found", i)
			}

			lastContent := res[len(res)-1]
			if lastContent.Role != genai.RoleModel {
				// it's okay, if model has not anything to say
				// we just add empty content
				res = append(res, &genai.Content{
					Role: genai.RoleModel,
				})
				lastContent = res[len(res)-1]
			}

			lastContent.Parts = append(lastContent.Parts, genai.NewPartFromFunctionResponse(m.ToolName(), map[string]any{
				"output": m.Content(),
			}))

		case messages.MessageToolError:
			if len(res) == 0 {
				return nil, fmt.Errorf("tool error message %v is orphaned, no assistant message found", i)
			}

			lastContent := res[len(res)-1]
			if lastContent.Role != genai.RoleModel {
				// it's okay, if model has not anything to say
				// we just add empty content
				res = append(res, &genai.Content{
					Role: genai.RoleModel,
				})
				lastContent = res[len(res)-1]
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

func toolInfoToGenAI(tools []tools.RawToolInfo) (res []*genai.Tool) {
	decls := make([]*genai.FunctionDeclaration, len(tools))
	for i, tool := range tools {
		decls[i] = &genai.FunctionDeclaration{
			Name:                 tool.Name(),
			Description:          tool.Desc(),
			ParametersJsonSchema: tool.Params(),
			ResponseJsonSchema:   tool.Response(),
		}
	}

	return []*genai.Tool{{
		FunctionDeclarations: decls,
	}}
}

package gemini

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/genai"

	"tg-helper/internal/domains/components/messages"
)

func MessageFromGenAIContent(resp *genai.GenerateContentResponse, thoughtBuffer string, mergeTag uint64) (res []messages.Message, thoughtsLeft string, err error) {
	if resp == nil || len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return nil, "", fmt.Errorf("received empty response from model")
	}

	if len(resp.Candidates) > 1 {
		return nil, "", fmt.Errorf("multiple candidates are not supported, got %d candidates", len(resp.Candidates))
	}

	candidate := resp.Candidates[0]
	if candidate.Content == nil {
		return nil, "", fmt.Errorf("candidate content is nil")
	}

	switch candidate.Content.Role {
	case genai.RoleModel:
		parts := candidate.Content.Parts
		if len(parts) == 0 {
			return nil, "", fmt.Errorf("candidate content has no parts")
		}

		for _, part := range parts {
			switch {
			case part.Text != "":
				if part.Thought {
					thoughtBuffer += part.Text
				} else {
					msg, err := messages.NewMessageAssistant(part.Text,
						messages.WithMessageAssistantMergeTag(mergeTag),
						messages.WithMessageAssistantReasoning(thoughtBuffer),
					)
					if err != nil {
						return nil, "", fmt.Errorf("failed to create assistant message: %w", err)
					}
					res = append(res, msg)
					thoughtBuffer = "" // reset thought buffer after sending a message
				}

			case part.FunctionCall != nil:
				call := part.FunctionCall

				args := make(map[string]json.RawMessage)
				for k, v := range call.Args {
					if args[k], err = json.Marshal(v); err != nil {
						return nil, "", fmt.Errorf("failed to marshal function call argument %q: %w", k, err)
					}
				}

				if call.ID == "" {
					// call ID should not be empty, we have to generate it
					call.ID = uuid.NewString()
				}

				msg, err := messages.NewMessageToolRequest(args, call.Name, call.ID,
					messages.WithMessageToolRequestMergeTag(mergeTag),
					messages.WithMessageToolRequestReasoning(thoughtBuffer),
				)
				if err != nil {
					return nil, "", fmt.Errorf("failed to create tool request message: %w", err)
				}
				res = append(res, msg)
			case part.FunctionResponse != nil:
				resp := part.FunctionResponse

				var output any = resp.Response
				if out, ok := resp.Response["output"]; ok {
					output = out
				}

				response, err := json.Marshal(output)
				if err != nil {
					return nil, "", fmt.Errorf("failed to marshal function response output: %w", err)
				}

				msg, err := messages.NewMessageToolResponse(response, resp.Name, resp.ID)
				if err != nil {
					return nil, "", fmt.Errorf("failed to create tool response message: %w", err)
				}
				res = append(res, msg)

			case part.FileData != nil:
				return nil, "", fmt.Errorf("file data is not yet supported, unfortunately")

			case part.ExecutableCode != nil, part.CodeExecutionResult != nil, part.VideoMetadata != nil:
				return nil, "", fmt.Errorf("content is not supported")

			// NOTE: thought signature is not provided in streaming. Looks
			// like it's an embedding for next reasoning. So, looks like
			// Gemini doesn't work with thoughts on input side.
			default:
				data, err := json.Marshal(part)
				if err != nil {
					return nil, "", fmt.Errorf("failed to marshal part: %w", err)
				}

				return nil, "", fmt.Errorf("unexpected part type in candidate content: %s", string(data))
			}
		}

		return res, thoughtBuffer, nil

	case genai.RoleUser:
		return nil, "", fmt.Errorf("user role is not expected in candidate content")
	default:
		return nil, "", fmt.Errorf("unexpected role %q in candidate content", candidate.Content.Role)
	}
}

func ptr[T any](v T) *T { return &v }

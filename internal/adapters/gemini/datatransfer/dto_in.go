package datatransfer

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/genai"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

func MessageFromGenAIContent(resp *genai.GenerateContentResponse, thoughtBuffer string, metadataBuffer []byte, mergeTag uint64, agentID ids.AgentID) (res []messages.Message, thoughtsLeft string, metadataLeft []byte, err error) {
	if resp == nil || len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return nil, "", nil, fmt.Errorf("received empty response from model")
	}

	if len(resp.Candidates) > 1 {
		return nil, "", nil, fmt.Errorf("multiple candidates are not supported, got %d candidates", len(resp.Candidates))
	}

	candidate := resp.Candidates[0]
	if candidate.Content == nil {
		return nil, "", nil, fmt.Errorf("candidate content is nil")
	}

	switch candidate.Content.Role {
	case genai.RoleModel, "":
		parts := candidate.Content.Parts
		if len(parts) == 0 {
			return nil, "", nil, fmt.Errorf("candidate content has no parts")
		}

		currentMetadata := metadataBuffer
		for _, part := range parts {
			// Extract thought signature if present in this part.
			// Gemini often includes this in the same part as the function call or reasoning.
			if part.ThoughtSignature != nil {
				sig, err := json.Marshal(struct {
					Sig []byte `json:"gemini_thought_signature"`
				}{
					Sig: part.ThoughtSignature,
				})
				if err != nil {
					return nil, "", nil, fmt.Errorf("failed to marshal thought signature: %w", err)
				}
				currentMetadata = sig
			}

			switch {
			case part.Text != "" || part.Thought:
				if part.Thought {
					thoughtBuffer += part.Text
				} else {
					msg, err := messages.NewMessageAssistant(part.Text,
						messages.WithMessageAssistantMergeTag(mergeTag),
						messages.WithMessageAssistantReasoning(thoughtBuffer),
						messages.WithMessageAssistantAgentID(agentID),
						messages.WithMessageAssistantProtocolMetadata(currentMetadata),
					)
					if err != nil {
						return nil, "", nil, fmt.Errorf("failed to create assistant message: %w", err)
					}
					res = append(res, msg)
					thoughtBuffer = "" // reset thought buffer after sending a message
				}

			case part.FunctionCall != nil:
				call := part.FunctionCall

				args := make(map[string]json.RawMessage)
				for k, v := range call.Args {
					if args[k], err = json.Marshal(v); err != nil {
						return nil, "", nil, fmt.Errorf("failed to marshal function call argument %q: %w", k, err)
					}
				}
				if call.Name == "" {
					return nil, "", nil, fmt.Errorf("function call has no name")
				}

				if call.ID == "" {
					// call ID should not be empty, we have to generate it
					call.ID = uuid.NewString()
				}

				msg, err := messages.NewMessageToolRequest(args, call.Name, call.ID,
					messages.WithMessageToolRequestMergeTag(mergeTag),
					messages.WithMessageToolRequestReasoning(thoughtBuffer),
					messages.WithMessageToolRequestProtocolMetadata(currentMetadata),
				)
				if err != nil {
					return nil, "", nil, fmt.Errorf("failed to create tool request message: %w", err)
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
					return nil, "", nil, fmt.Errorf("failed to marshal function response output: %w", err)
				}

				msg, err := messages.NewMessageToolResponse(response, resp.Name, resp.ID)
				if err != nil {
					return nil, "", nil, fmt.Errorf("failed to create tool response message: %w", err)
				}
				res = append(res, msg)

			case part.FileData != nil:
				return nil, "", nil, fmt.Errorf("file data is not yet supported, unfortunately")

			case part.ExecutableCode != nil, part.CodeExecutionResult != nil, part.VideoMetadata != nil:
				return nil, "", nil, fmt.Errorf("content is not supported")

			case part.ThoughtSignature != nil && part.Text == "" && !part.Thought && part.FunctionCall == nil:
				// This part only contained a signature, which we already handled above.
				continue

			//TODO: handle artifacts through default.
			case part.Text == "" && part.FunctionCall == nil && part.FunctionResponse == nil:
				// Skip empty parts (metadata or stream artifacts)
				continue

			default:
				data, err := json.Marshal(part)
				if err != nil {
					return nil, "", nil, fmt.Errorf("failed to marshal part: %w", err)
				}

				return nil, "", nil, fmt.Errorf("unexpected part type in candidate content: %s", string(data))
			}
		}

		return res, thoughtBuffer, currentMetadata, nil

	case genai.RoleUser:
		return nil, "", nil, fmt.Errorf("user role is not expected in candidate content")
	default:
		return nil, "", nil, fmt.Errorf("unexpected role %q in candidate content", candidate.Content.Role)
	}
}

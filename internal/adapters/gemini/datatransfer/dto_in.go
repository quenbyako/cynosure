package datatransfer

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/genai"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

// MessageFromGenAIContent converts Gemini response to internal messages.
//
//nolint:gocritic // responging 4 types by intention
func MessageFromGenAIContent(
	resp *genai.GenerateContentResponse,
	thoughtBuffer string,
	metadataBuffer []byte,
	mergeTag uint64,
	agentID ids.AgentID,
) (res []messages.Message, thoughtsLeft string, metadataLeft []byte, err error) {
	if resp == nil || len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return nil, "", nil, ErrEmptyResponse
	}

	if len(resp.Candidates) > 1 {
		return nil, "", nil, fmt.Errorf("%w: got %d", ErrMultipleCandidates, len(resp.Candidates))
	}

	candidate := resp.Candidates[0]
	if candidate.Content == nil {
		return nil, "", nil, ErrCandidateContentNil
	}

	return processCandidate(candidate, thoughtBuffer, metadataBuffer, mergeTag, agentID)
}

//nolint:gocritic // responging 4 types by intention
func processCandidate(
	candidate *genai.Candidate,
	thought string,
	metadata []byte,
	tag uint64,
	agentID ids.AgentID,
) ([]messages.Message, string, []byte, error) {
	switch candidate.Content.Role {
	case genai.RoleModel, "":
		return processModelParts(candidate.Content.Parts, thought, metadata, tag, agentID)
	case genai.RoleUser:
		return nil, "", nil, ErrUnexpectedRole
	default:
		return nil, "", nil, fmt.Errorf("%w: %q", ErrUnexpectedRole, candidate.Content.Role)
	}
}

//nolint:gocritic // responging 4 types by intention
func processModelParts(
	parts []*genai.Part,
	thought string,
	metadata []byte,
	tag uint64,
	agentID ids.AgentID,
) (res []messages.Message, thoughtsLeft string, metadataLeft []byte, err error) {
	if len(parts) == 0 {
		return nil, thought, metadata, nil
	}

	curMeta, curThought := metadata, thought

	for _, part := range parts {
		if part.ThoughtSignature != nil {
			if curMeta, err = marshalThoughtSig(part.ThoughtSignature); err != nil {
				return nil, "", nil, err
			}
		}

		var msgs []messages.Message

		msgs, curThought, err = processPart(part, curThought, curMeta, tag, agentID)
		if err != nil {
			return nil, "", nil, err
		}

		res = append(res, msgs...)
	}

	return res, curThought, curMeta, nil
}

func marshalThoughtSig(sig []byte) ([]byte, error) {
	wrapped := struct {
		Sig []byte `json:"gemini_thought_signature"`
	}{Sig: sig}

	data, err := json.Marshal(wrapped)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal thought signature: %w", err)
	}

	return data, nil
}

func processPart(
	part *genai.Part,
	thought string,
	metadata []byte,
	tag uint64,
	agentID ids.AgentID,
) ([]messages.Message, string, error) {
	switch {
	case part.Text != "" || part.Thought:
		return processTextPart(part, thought, metadata, tag, agentID)
	case part.FunctionCall != nil:
		msg, err := processFuncCall(part.FunctionCall, thought, metadata, tag)
		return []messages.Message{msg}, thought, err
	case part.FunctionResponse != nil:
		msg, err := processFuncResp(part.FunctionResponse)
		return []messages.Message{msg}, thought, err
	default:
		return processMiscPart(part, thought)
	}
}

func processMiscPart(part *genai.Part, thought string) ([]messages.Message, string, error) {
	switch {
	case part.FileData != nil:
		return nil, "", ErrFileDataUnsupported
	case part.ExecutableCode != nil || part.CodeExecutionResult != nil || part.VideoMetadata != nil:
		return nil, "", ErrContentUnsupported
	case isMetadataPart(part):
		return nil, thought, nil
	default:
		return nil, "", unexpectedPartError(part)
	}
}

func processTextPart(
	part *genai.Part,
	thought string,
	meta []byte,
	tag uint64,
	agentID ids.AgentID,
) ([]messages.Message, string, error) {
	if part.Thought {
		return nil, thought + part.Text, nil
	}

	msg, err := messages.NewMessageAssistant(
		part.Text,
		messages.WithMessageAssistantMergeTag(tag),
		messages.WithMessageAssistantReasoning(thought),
		messages.WithMessageAssistantAgentID(agentID),
		messages.WithMessageAssistantProtocolMetadata(meta),
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create assistant message: %w", err)
	}

	return []messages.Message{msg}, "", nil
}

//nolint:ireturn // conversion to domain object
func processFuncCall(
	call *genai.FunctionCall,
	thought string,
	metadata []byte,
	tag uint64,
) (messages.Message, error) {
	args, err := marshalArgs(call.Args)
	if err != nil {
		return nil, err
	}

	if call.Name == "" {
		return nil, ErrFunctionCallNoName
	}

	callID := call.ID
	if callID == "" {
		callID = uuid.NewString()
	}

	msg, err := messages.NewMessageToolRequest(args, call.Name, callID,
		messages.WithMessageToolRequestMergeTag(tag),
		messages.WithMessageToolRequestReasoning(thought),
		messages.WithMessageToolRequestProtocolMetadata(metadata),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create tool request message: %w", err)
	}

	return msg, nil
}

func marshalArgs(rawArgs map[string]any) (map[string]json.RawMessage, error) {
	args := make(map[string]json.RawMessage)

	for key, val := range rawArgs {
		data, err := json.Marshal(val)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal function call argument %q: %w", key, err)
		}

		args[key] = data
	}

	return args, nil
}

//nolint:ireturn // conversion to domain object
func processFuncResp(resp *genai.FunctionResponse) (messages.Message, error) {
	var output any = resp.Response
	if out, ok := resp.Response["output"]; ok {
		output = out
	}

	responseData, err := json.Marshal(output)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal function response output: %w", err)
	}

	msg, err := messages.NewMessageToolResponse(responseData, resp.Name, resp.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to create tool response message: %w", err)
	}

	return msg, nil
}

func isMetadataPart(part *genai.Part) bool {
	if part.ThoughtSignature != nil && part.Text == "" && !part.Thought &&
		part.FunctionCall == nil {
		return true
	}

	if part.Text == "" && part.FunctionCall == nil && part.FunctionResponse == nil {
		return true
	}

	return false
}

func unexpectedPartError(part *genai.Part) error {
	data, err := json.Marshal(part)
	if err != nil {
		return fmt.Errorf("failed to marshal part: %w", err)
	}

	return UnexpectedPartError(string(data))
}

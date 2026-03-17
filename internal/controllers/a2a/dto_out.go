package a2a

import (
	"encoding/json"
	"fmt"

	"google.golang.org/a2a"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

func messagesTo(msg messages.Message) (res *a2a.Message, err error) {
	switch msg := msg.(type) {
	case messages.MessageAssistant:
		return assistantMessage(msg), nil

	case messages.MessageToolRequest:
		return toolRequestMessage(msg)

	case messages.MessageToolResponse:
		return toolResponseMessage(msg)

	default:
		return nil, ErrInternalValidation("unknown message type: %T", msg)
	}
}

func assistantMessage(msg messages.MessageAssistant) *a2a.Message {
	return &a2a.Message{
		Role: a2a.Role_ROLE_AGENT,
		Content: []*a2a.Part{{
			Part: &a2a.Part_Text{
				Text: msg.Content(),
			},
		}},
		MessageId:  "",
		ContextId:  "",
		TaskId:     "",
		Metadata:   nil,
		Extensions: nil,
	}
}

func toolRequestMessage(msg messages.MessageToolRequest) (*a2a.Message, error) {
	args, err := argsToStruct(msg.Arguments())
	if err != nil {
		return nil, err
	}

	return &a2a.Message{
		Role: a2a.Role_ROLE_AGENT,
		Content: []*a2a.Part{{
			Part: &a2a.Part_Data{
				Data: &a2a.DataPart{Data: args},
			},
		}},
		Metadata: &structpb.Struct{Fields: map[string]*structpb.Value{
			"tool":   structpb.NewStringValue(msg.ToolName()),
			"reason": structpb.NewStringValue("Invoking tool"),
		}},
		MessageId:  "",
		ContextId:  "",
		TaskId:     "",
		Extensions: nil,
	}, nil
}

func toolResponseMessage(msg messages.MessageToolResponse) (*a2a.Message, error) {
	var content any
	if err := json.Unmarshal(msg.Content(), &content); err != nil {
		return nil, fmt.Errorf("unmarshalling arg: %w", err)
	}

	value, err := structpb.NewValue(content)
	if err != nil {
		return nil, fmt.Errorf("creating struct for content: %w", err)
	}

	return &a2a.Message{
		Role: a2a.Role_ROLE_USER,
		Content: []*a2a.Part{{
			Part: &a2a.Part_Data{
				Data: &a2a.DataPart{Data: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"response": value,
					},
				}},
			},
		}},
		Metadata: &structpb.Struct{Fields: map[string]*structpb.Value{
			"tool":   structpb.NewStringValue(msg.ToolName()),
			"reason": structpb.NewStringValue("Invoking tool"),
		}},
		MessageId:  "",
		ContextId:  "",
		TaskId:     "",
		Extensions: nil,
	}, nil
}

func argsToStruct(args map[string]json.RawMessage) (*structpb.Struct, error) {
	argsRaw := make(map[string]any, len(args))
	for key, value := range args {
		var x any
		if err := json.Unmarshal(value, &x); err != nil {
			return nil, fmt.Errorf("unmarshalling arg %q: %w", key, err)
		}

		argsRaw[key] = x
	}

	res, err := structpb.NewStruct(argsRaw)
	if err != nil {
		return nil, fmt.Errorf("creating struct for args: %w", err)
	}

	return res, nil
}

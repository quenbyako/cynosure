package a2a

import (
	"encoding/json"
	"fmt"

	"google.golang.org/a2a"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/messages"
)

func messagesTo(m messages.Message) (res *a2a.Message, err error) {
	switch m := m.(type) {
	case messages.MessageAssistant:
		return &a2a.Message{
			Role: a2a.Role_ROLE_AGENT,
			Content: []*a2a.Part{{
				Part: &a2a.Part_Text{
					Text: m.Text(),
				},
			}},
		}, nil

	case messages.MessageToolRequest:
		argsRaw := make(map[string]any, len(m.Arguments()))
		for k, v := range m.Arguments() {
			var x any
			if err := json.Unmarshal(v, &x); err != nil {
				return nil, fmt.Errorf("unmarshalling arg %q: %w", k, err)
			}
			argsRaw[k] = x
		}
		args, err := structpb.NewStruct(argsRaw)
		if err != nil {
			return nil, fmt.Errorf("creating struct for args: %w", err)
		}

		return &a2a.Message{
			Role: a2a.Role_ROLE_AGENT,
			Content: []*a2a.Part{{
				Part: &a2a.Part_Data{
					Data: &a2a.DataPart{Data: args},
				},
			}},
			Metadata: &structpb.Struct{Fields: map[string]*structpb.Value{
				"tool":   structpb.NewStringValue(m.ToolName()),
				"reason": structpb.NewStringValue("Invoking tool"),
			}},
		}, nil

	case messages.MessageToolResponse:
		var content any
		if err := json.Unmarshal(m.Content(), &content); err != nil {
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
				"tool":   structpb.NewStringValue(m.ToolName()),
				"reason": structpb.NewStringValue("Invoking tool"),
			}},
		}, nil

	default:
		return nil, fmt.Errorf("unknown message type: %T", m)
	}
}

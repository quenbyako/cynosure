package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"math/rand/v2"

	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.39.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/genai"

	"github.com/quenbyako/cynosure/internal/adapters/gemini/datatransfer"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
)

type GenAIInputMessagesAttribute []ChatMessage

type ChatMessage struct {
	Name  *string                           `json:"name,omitempty"`
	Role  Role                              `json:"role"`
	Parts []GenAIInputMessagesAttributePart `json:"parts"`
}

type GenAIInputMessagesAttributePart interface {
	_GenAIInputMessagesAttributePart()
}

type TextPart struct {
	Type    string `json:"type"` // always "text"
	Content string `json:"content"`
}

func (*TextPart) _GenAIInputMessagesAttributePart() {}

type ToolCallRequestPart struct {
	Type      string          `json:"type"` // always "tool_call"
	Id        *string         `json:"id,omitempty"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

func (*ToolCallRequestPart) _GenAIInputMessagesAttributePart() {}

type ToolCallResponsePart struct {
	Type     string          `json:"type"` // always "tool_call_response"
	Id       *string         `json:"id,omitempty"`
	Name     string          `json:"name"`
	Response json.RawMessage `json:"response,omitempty"`
}

func (*ToolCallResponsePart) _GenAIInputMessagesAttributePart() {}

func otelMessageFromMessage(systemMsg string, msgs []messages.Message) []ChatMessage {
	res := make([]ChatMessage, 0, len(msgs))

	if systemMsg != "" {
		res = append(res, ChatMessage{
			Role: RoleSystem,
			Parts: []GenAIInputMessagesAttributePart{
				&TextPart{
					Type:    "text",
					Content: systemMsg,
				},
			},
		})
	}

	var this *ChatMessage

	for _, msg := range msgs {
		switch msg := msg.(type) {
		case messages.MessageAssistant:
			if this != nil && this.Role != RoleAssistant {
				res, this = append(res, *this), nil
			}

			if this == nil {
				this = &ChatMessage{Role: RoleAssistant}
			}

			this.Parts = append(this.Parts, &TextPart{
				Type:    "text",
				Content: msg.Content(),
			})
		case messages.MessageTool:
			if this != nil && this.Role != RoleTool {
				res, this = append(res, *this), nil
			}

			if this == nil {
				this = &ChatMessage{Role: RoleTool}
			}

			this.Parts = append(this.Parts, &ToolCallResponsePart{
				Type:     "tool_call_response",
				Id:       ptr(msg.ToolCallID()),
				Name:     msg.ToolName(),
				Response: msg.Content(),
			})
		case messages.MessageToolRequest:
			if this != nil && this.Role != RoleAssistant {
				res, this = append(res, *this), nil
			}

			if this == nil {
				this = &ChatMessage{Role: RoleAssistant}
			}

			this.Parts = append(this.Parts, &ToolCallRequestPart{
				Type:      "tool_call",
				Id:        ptr(msg.ToolCallID()),
				Name:      msg.ToolName(),
				Arguments: must(json.Marshal(msg.Arguments())),
			})
		case messages.MessageUser:
			if this != nil && this.Role != RoleUser {
				res, this = append(res, *this), nil
			}

			if this == nil {
				this = &ChatMessage{Role: RoleUser}
			}

			this.Parts = append(this.Parts, &TextPart{
				Type:    "text",
				Content: msg.Content(),
			})
		default:
			panic(fmt.Sprintf("unexpected messages.Message: %#v", msg))
		}
	}

	if this != nil {
		res = append(res, *this)
	}

	return res
}

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Stream implements [ports.ChatModel].
func (g *GeminiModel) Stream(ctx context.Context, input []messages.Message, settings entities.AgentReadOnly, opts ...ports.StreamOption) (iter.Seq2[messages.Message, error], error) {
	msgsJSON := string(must(json.Marshal(otelMessageFromMessage(settings.SystemMessage(), input))))

	attrs := []attribute.KeyValue{
		semconv.GenAIOperationNameGenerateContent,
		semconv.GenAIProviderNameGCPGemini,
		semconv.GenAIConversationID("TODO"),
		semconv.GenAIRequestModel(settings.Model()),
		semconv.GenAIInputMessagesKey.String(msgsJSON),
	}
	if v, ok := settings.TopP(); ok {
		attrs = append(attrs, semconv.GenAIRequestTopP(float64(v)))
	}

	if v, ok := settings.Temperature(); ok {
		attrs = append(attrs, semconv.GenAIRequestTemperature(float64(v)))
	}

	ctx, span := g.trace.Start(ctx, "GeminiModel.Stream", trace.WithAttributes(attrs...))
	defer span.End()

	p := ports.StreamParams(opts...)

	genConfig := &genai.GenerateContentConfig{
		ThinkingConfig: g.thinkingConfig,
	}

	if systemMessage := settings.SystemMessage(); systemMessage != "" {
		genConfig.SystemInstruction = &genai.Content{
			Role:  "", // role is not used for system instructions
			Parts: []*genai.Part{genai.NewPartFromText(systemMessage)},
		}
	}

	toolList := p.Toolbox().List()
	if len(toolList) > 0 {
		var mode genai.FunctionCallingConfigMode

		switch toolChoice := p.ToolChoice(); toolChoice {
		case tools.ToolChoiceAllowed:
			mode = genai.FunctionCallingConfigModeAuto
		case tools.ToolChoiceForced:
			mode = genai.FunctionCallingConfigModeAny
		case tools.ToolChoiceForbidden:
			mode = genai.FunctionCallingConfigModeNone
		default:
			return nil, fmt.Errorf("unknown tool choice: %v", toolChoice)
		}

		genConfig.ToolConfig = &genai.ToolConfig{
			FunctionCallingConfig: &genai.FunctionCallingConfig{
				Mode: mode,
			},
		}

		genConfig.Tools = datatransfer.ToolInfoToGenAI(toolList)
	}

	g.log.GeminiStreamStarted(ctx, settings.Model(), len(toolList))

	converted, err := datatransfer.MessagesToGenAIContent(input)
	if err != nil {
		return nil, fmt.Errorf("failed to convert messages: %w", err)
	}

	s := g.client.Models.GenerateContentStream(ctx, settings.Model(), converted, genConfig)

	mergeTag := rand.Uint64()

	return func(yield func(messages.Message, error) bool) {
		var (
			thoughtBuffer  string
			metadataBuffer []byte
		)

		mapper := func(msg *genai.GenerateContentResponse, err error) (res []messages.Message, _ error) {
			if err != nil {
				return nil, err
			}

			res, thoughtBuffer, metadataBuffer, err = datatransfer.MessageFromGenAIContent(msg, thoughtBuffer, metadataBuffer, mergeTag, settings.ID())
			if err != nil {
				return nil, fmt.Errorf("failed to convert message from Gemini: %w", err)
			}

			return res, nil
		}

		IterExtract(SafeMap(s, mapper))(yield)
	}, nil
}

// SafeMap wraps an iterator with a mapper function and ensures that yield
// is never called again after it returns false, or after an error occurs.
func SafeMap[K1, V1, K2 any](seq iter.Seq2[K1, V1], mapper func(K1, V1) (K2, error)) iter.Seq2[K2, error] {
	return func(yield func(K2, error) bool) {
		seq(func(k1 K1, v1 V1) bool {
			k2, err := mapper(k1, v1)
			if err != nil {
				yield(k2, err)
				return false
			}

			return yield(k2, err)
		})
	}
}

// IterExtract flattens an iterator of slices into an iterator of elements.
func IterExtract[K1 any](seq iter.Seq2[[]K1, error]) iter.Seq2[K1, error] {
	return func(yield func(K1, error) bool) {
		seq(func(k1 []K1, err error) bool {
			if err != nil {
				yield(*new(K1), err)
				return false
			}

			for _, item := range k1 {
				if !yield(item, nil) {
					return false
				}
			}

			return true
		})
	}
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}

	return v
}

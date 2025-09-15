package gemini

import (
	"context"
	"fmt"
	"iter"
	"math/rand/v2"

	"google.golang.org/genai"

	"tg-helper/internal/domains/components/messages"
	"tg-helper/internal/domains/components/tools"
	"tg-helper/internal/domains/entities"
	"tg-helper/internal/domains/ports"
)

type ClientConfig = genai.ClientConfig

type GeminiModel struct {
	client *genai.Client
	model  string

	thinkingConfig *genai.ThinkingConfig
}

var _ ports.ChatModel = (*GeminiModel)(nil)

func NewGeminiModel(ctx context.Context, model string, cfg *ClientConfig) (*GeminiModel, error) {
	client, err := genai.NewClient(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create GenAI client: %w", err)
	}

	return &GeminiModel{
		client: client,
		model:  model,
		/**/
		thinkingConfig: &genai.ThinkingConfig{
			IncludeThoughts: true,
			ThinkingBudget:  ptr[int32](32),
		},
		/**/
	}, nil
}

// Stream implements adapters.ChatModel.
func (g *GeminiModel) Stream(ctx context.Context, input []messages.Message, settings entities.ModelSettingsReadOnly, opts ...ports.StreamOption) (iter.Seq2[messages.Message, error], error) {
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

	if t := p.Tools(); len(t) > 0 {
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

		genConfig.Tools = toolInfoToGenAI(t)
	}

	converted, err := messagesToGenAIContent(input)
	if err != nil {
		return nil, fmt.Errorf("failed to convert messages: %w", err)
	}

	s := g.client.Models.GenerateContentStream(ctx, g.model, converted, genConfig)

	mergeTag := rand.Uint64()

	return func(yield func(messages.Message, error) bool) {
		var thoughtBuffer string

		s(func(msg *genai.GenerateContentResponse, err error) bool {
			if err != nil {
				// Check if the error is due to context cancellation, which is expected when the loop breaks.
				if ctx.Err() != nil {
					return false
				}

				return yield(nil, fmt.Errorf("failed to generate content: %w", err))
			}

			var res []messages.Message
			res, thoughtBuffer, err = MessageFromGenAIContent(msg, thoughtBuffer, mergeTag)
			if err != nil {
				yield(nil, err)

				return false // no matter what — stop iteration on error
			}

			for _, m := range res {
				// pp.Println(m) // Debugging output, can be removed later

				if !yield(m, nil) {
					return false // stop iteration if yield returns false
				}
			}

			return true
		})
	}, nil
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

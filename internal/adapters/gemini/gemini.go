package gemini

import (
	"context"
	"fmt"
	"iter"
	"math/rand/v2"

	"google.golang.org/genai"

	"github.com/quenbyako/cynosure/internal/adapters/gemini/datatransfer"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
)

type ClientConfig = genai.ClientConfig

type GeminiModel struct {
	client *genai.Client
	log    LogCallbacks

	thinkingConfig *genai.ThinkingConfig
}

var _ ports.ChatModelFactory = (*GeminiModel)(nil)
var _ ports.ToolSemanticIndexFactory = (*GeminiModel)(nil)

func (g *GeminiModel) ChatModel() ports.ChatModel { return g }

type GeminiModelOption func(*GeminiModel)

func WithLogCallbacks(log LogCallbacks) GeminiModelOption {
	return func(g *GeminiModel) { g.log = log }
}

func NewGeminiModel(ctx context.Context, cfg *ClientConfig, opts ...GeminiModelOption) (*GeminiModel, error) {
	client, err := genai.NewClient(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create GenAI client: %w", err)
	}

	m := GeminiModel{
		client: client,
		log:    NoOpLogCallbacks{},
		/**/
		thinkingConfig: &genai.ThinkingConfig{
			IncludeThoughts: true,
			ThinkingBudget:  ptr[int32](32),
		},
		/**/
	}
	for _, opt := range opts {
		opt(&m)
	}

	return &m, nil
}

// Stream implements adapters.ChatModel.
func (g *GeminiModel) Stream(ctx context.Context, input []messages.Message, settings entities.AgentReadOnly, opts ...ports.StreamOption) (iter.Seq2[messages.Message, error], error) {
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
		var thoughtBuffer string
		var metadataBuffer []byte

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

func ptr[T any](v T) *T { return &v }

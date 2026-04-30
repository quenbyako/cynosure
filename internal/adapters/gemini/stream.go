package gemini

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"iter"

	"google.golang.org/genai"

	"github.com/quenbyako/cynosure/internal/adapters/gemini/datatransfer"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/chatmodel"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
)

// Stream implements ports.ChatModel.
func (g *GeminiModel) Stream(
	ctx context.Context,
	input []messages.Message,
	settings entities.AgentReadOnly,
	opts ...chatmodel.StreamOption,
) (iter.Seq2[messages.Message, error], error) {
	if uint(len(input)) > g.hardCap {
		return nil, chatmodel.ErrHistoryTooLong
	}

	params, err := chatmodel.StreamParams(input, settings, opts...)
	if err != nil {
		return nil, err
	}

	genConfig, err := g.buildGenConfig(params.Settings(), &params)
	if err != nil {
		return nil, err
	}

	g.log.GeminiStreamStarted(ctx, params.Settings().Model(), len(params.Toolbox().List()))

	converted, err := datatransfer.MessagesToGenAIContent(params.Input())
	if err != nil {
		return nil, fmt.Errorf("failed to convert messages: %w", err)
	}

	stream := g.client.Models.GenerateContentStream(ctx, params.Settings().Model(), converted, genConfig)
	tag := randomUint64()

	return func(yield func(messages.Message, error) bool) {
		g.streamIter(yield, stream, tag, settings)
	}, nil
}

func (g *GeminiModel) streamIter(
	yield func(messages.Message, error) bool,
	stream iter.Seq2[*genai.GenerateContentResponse, error],
	tag uint64,
	settings entities.AgentReadOnly,
) {
	var (
		thought  string
		metadata []byte
	)

	mapper := func(msg *genai.GenerateContentResponse, err error) ([]messages.Message, error) {
		if err != nil {
			return nil, err
		}

		var res []messages.Message

		res, thought, metadata, err = datatransfer.MessageFromGenAIContent(
			msg, thought, metadata, tag, settings.ID(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to convert message from Gemini: %w", err)
		}

		return res, nil
	}

	IterExtract(SafeMap(stream, mapper))(yield)
}

func (g *GeminiModel) buildGenConfig(
	settings entities.AgentReadOnly,
	params streamParamsProxy,
) (*genai.GenerateContentConfig, error) {
	config := emptyConfig(g.thinkingConfig)

	if msg := settings.SystemMessage(); msg != "" {
		config.SystemInstruction = systemInstruction(msg)
	}

	toolList := params.Toolbox().List()
	if len(toolList) > 0 {
		mode, err := convertToolChoice(params.ToolChoice())
		if err != nil {
			return nil, err
		}

		config.ToolConfig = toolConfig(mode)
		config.Tools = datatransfer.ToolInfoToGenAI(toolList)
	}

	return config, nil
}

func emptyConfig(thinking *genai.ThinkingConfig) *genai.GenerateContentConfig {
	var config genai.GenerateContentConfig

	config.ThinkingConfig = thinking

	return &config
}

func systemInstruction(msg string) *genai.Content {
	return &genai.Content{
		Parts: []*genai.Part{{
			Text:                msg,
			MediaResolution:     nil,
			CodeExecutionResult: nil,
			ExecutableCode:      nil,
			FileData:            nil,
			FunctionCall:        nil,
			FunctionResponse:    nil,
			InlineData:          nil,
			Thought:             false,
			ThoughtSignature:    nil,
			VideoMetadata:       nil,
		}},
		Role: "",
	}
}

func toolConfig(mode genai.FunctionCallingConfigMode) *genai.ToolConfig {
	return &genai.ToolConfig{
		FunctionCallingConfig: &genai.FunctionCallingConfig{
			Mode:                        mode,
			AllowedFunctionNames:        nil,
			StreamFunctionCallArguments: nil,
		},
		RetrievalConfig:                  nil,
		IncludeServerSideToolInvocations: nil,
	}
}

type streamParamsProxy interface {
	Toolbox() tools.Toolbox
	ToolChoice() tools.ToolChoice
}

func convertToolChoice(choice tools.ToolChoice) (genai.FunctionCallingConfigMode, error) {
	switch choice {
	case tools.ToolChoiceAllowed:
		return genai.FunctionCallingConfigModeAuto, nil
	case tools.ToolChoiceForced:
		return genai.FunctionCallingConfigModeAny, nil
	case tools.ToolChoiceForbidden:
		return genai.FunctionCallingConfigModeNone, nil
	default:
		return "", fmt.Errorf("%w: %v", ErrUnknownToolChoice, choice)
	}
}

func randomUint64() uint64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0
	}

	return binary.LittleEndian.Uint64(b[:])
}

package gemini

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"google.golang.org/genai"

	"github.com/quenbyako/cynosure/internal/adapters/gemini/datatransfer"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/chatmodel"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
)

func (g *GeminiModel) StreamWithStats(
	ctx context.Context,
	input []messages.Message,
	settings entities.AgentReadOnly,
	opts ...chatmodel.StreamOption,
) (chatmodel.Iter, error) {
	if uint(len(input)) > g.maxMsgsPerReq {
		return nil, chatmodel.ErrHistoryTooLong
	}

	params, err := chatmodel.StreamParams(input, settings, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare stream params: %w", err)
	}

	genConfig, err := g.buildGenConfig(params.Settings(), params.Toolbox(), params.ToolChoice())
	if err != nil {
		return nil, fmt.Errorf("failed to build genAI config: %w", err)
	}

	return g.executeStream(
		ctx,
		params.Settings(),
		params.PreflightCheck(),
		params.Toolbox(),
		params.Input(),
		genConfig,
	)
}

func (g *GeminiModel) prepareGenAIStream(
	ctx context.Context,
	settings entities.AgentReadOnly,
	preflight chatmodel.PreflightFunc,
	toolbox tools.Toolbox,
	input []messages.Message,
	genConfig *genai.GenerateContentConfig,
) ([]*genai.Content, int32, error) {
	g.obs.streamStarted(ctx, settings.Model(), len(toolbox.List()))

	converted, err := datatransfer.MessagesToGenAIContent(input)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to convert messages: %w", err)
	}

	totalTokens, err := g.countAndCheckTokens(
		ctx, settings.Model(), converted, genConfig, preflight,
	)
	if err != nil {
		return nil, 0, err
	}

	return converted, totalTokens, nil
}

func (g *GeminiModel) executeStream(
	ctx context.Context,
	settings entities.AgentReadOnly,
	preflight chatmodel.PreflightFunc,
	toolbox tools.Toolbox,
	input []messages.Message,
	genConfig *genai.GenerateContentConfig,
) (chatmodel.Iter, error) {
	converted, totalTokens, err := g.prepareGenAIStream(
		ctx, settings, preflight, toolbox, input, genConfig,
	)
	if err != nil {
		return nil, err
	}

	stream := g.client.Models.GenerateContentStream(
		ctx, settings.Model(), converted, genConfig,
	)

	session := g.newStreamSession(settings.ID())

	return NewIterCloser(
		stream,
		session.Map,
		session.Collect(ctx, settings.Model(), uint32(max(0, totalTokens))),
	), nil
}

func (g *GeminiModel) newStreamSession(agentID ids.AgentID) *geminiStreamSession {
	return &geminiStreamSession{
		g:         g,
		startTime: time.Now(),
		thought:   "",
		metadata:  nil,
		tag:       randomUint64(),
		agentID:   agentID,
	}
}

func (g *GeminiModel) countAndCheckTokens(
	ctx context.Context,
	model string,
	input []*genai.Content,
	genConfig *genai.GenerateContentConfig,
	preflight chatmodel.PreflightFunc,
) (int32, error) {
	content := getCountTokensContent(genConfig.SystemInstruction, input)

	tokens, err := g.client.Models.CountTokens(
		ctx, model, content, &genai.CountTokensConfig{
			// NOTE: for some silly reason, SystemInstruction is not supported
			// as field. Instead, we have to pass it in contents array.
			SystemInstruction: nil,
			// Tools are not supported in CountTokens for Gemini API too.
			Tools:            nil,
			HTTPOptions:      nil,
			GenerationConfig: nil,
		},
	)
	if err != nil {
		return 0, fmt.Errorf("token counting failed: %w", err)
	}

	if err := preflight(ctx, model, int(tokens.TotalTokens)); err != nil {
		return 0, chatmodel.ErrPreflightFailed(err)
	}

	return tokens.TotalTokens, g.waitInputLimit(ctx, int(tokens.TotalTokens))
}

func getCountTokensContent(systemMessage *genai.Content, input []*genai.Content) []*genai.Content {
	if systemMessage == nil || len(systemMessage.Parts) == 0 {
		return input
	}

	sysInstr := &genai.Content{
		Parts: systemMessage.Parts,
		// unlike GenerateContent, CountTokens does not treat empty role as system.
		Role: genai.RoleUser,
	}

	return append([]*genai.Content{sysInstr}, input...)
}

func (g *GeminiModel) waitInputLimit(ctx context.Context, numTokens int) error {
	cause := chatmodel.ErrHardQuotaExhausted
	limCtx, cancel := context.WithTimeoutCause(ctx, maxLimiterWait, cause)

	defer cancel()

	if err := g.chatInputLimiter.WaitN(limCtx, numTokens); err != nil {
		isCanceled := errors.Is(err, context.Canceled) ||
			errors.Is(context.Cause(limCtx), context.Canceled)

		if isCanceled {
			return err //nolint:wrapcheck
		}

		return chatmodel.ErrHardQuotaExhausted
	}

	return nil
}

type geminiStreamSession struct {
	g         *GeminiModel
	startTime time.Time
	thought   string
	metadata  []byte
	tag       uint64
	agentID   ids.AgentID
}

func (s *geminiStreamSession) Map(msg *genai.GenerateContentResponse) ([]messages.Message, error) {
	res, thought, meta, err := datatransfer.MessageFromGenAIContent(
		msg, s.thought, s.metadata, s.tag, s.agentID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to convert message from Gemini: %w", err)
	}

	s.thought = thought
	s.metadata = meta

	return res, nil
}

func (s *geminiStreamSession) Collect(
	ctx context.Context, model string, wantInputTokens uint32,
) func(chatmodel.UsageStats, *genai.GenerateContentResponse) chatmodel.UsageStats {
	return func(
		usage chatmodel.UsageStats, msg *genai.GenerateContentResponse,
	) chatmodel.UsageStats {
		if msg.UsageMetadata != nil {
			usage.InputTokens = uint32(max(0, msg.UsageMetadata.PromptTokenCount))
			usage.OutputTokens = uint32(max(0, msg.UsageMetadata.CandidatesTokenCount))

			if usage.InputTokens != wantInputTokens {
				s.g.obs.tokenCountMismatch(ctx, model, wantInputTokens, usage.InputTokens)
			}
		}

		usage.Duration = time.Since(s.startTime)

		return usage
	}
}

func (g *GeminiModel) buildGenConfig(
	settings entities.AgentReadOnly,
	toolbox tools.Toolbox,
	toolChoice tools.ToolChoice,
) (*genai.GenerateContentConfig, error) {
	config := emptyConfig(g.thinkingConfig)

	if msg := settings.SystemMessage(); msg != "" {
		config.SystemInstruction = systemInstruction(msg)
	}

	toolList := toolbox.List()
	if len(toolList) > 0 {
		mode, err := convertToolChoice(toolChoice)
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

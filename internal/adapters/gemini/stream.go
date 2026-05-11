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

	genConfig, err := g.buildGenConfig(params.Settings(), &params)
	if err != nil {
		return nil, fmt.Errorf("failed to build genAI config: %w", err)
	}

	g.log.GeminiStreamStarted(ctx, params.Settings().Model(), len(params.Toolbox().List()))

	converted, err := datatransfer.MessagesToGenAIContent(params.Input())
	if err != nil {
		return nil, fmt.Errorf("failed to convert messages: %w", err)
	}

	// custom context to prevent too long wait for rate limiter
	limiterCtx, cancel := context.WithTimeoutCause(ctx, maxLimiterWait, chatmodel.ErrHardQuotaExhausted)
	defer cancel()

	// TODO: use local counting, since we may have a big latency due to http
	// calls.
	tokens, err := g.client.Models.CountTokens(ctx, settings.Model(), converted, &genai.CountTokensConfig{
		HTTPOptions:       nil,
		SystemInstruction: genConfig.SystemInstruction,
		Tools:             genConfig.Tools,
		GenerationConfig:  nil,
	})
	if err != nil {
		return nil, fmt.Errorf("token counting failed: %w", err)
	}

	if err := params.PreflightCheck()(ctx, settings.Model(), int(tokens.TotalTokens)); err != nil {
		return nil, err // Abort request if rate limit exceeded
	}

	if err := g.chatInputLimiter.WaitN(limiterCtx, int(tokens.TotalTokens)); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(context.Cause(limiterCtx), context.Canceled) {
			return nil, err //nolint:wrapcheck // returning context cancellation as is
		}

		return nil, chatmodel.ErrHardQuotaExhausted
	}

	stream := g.client.Models.GenerateContentStream(ctx, params.Settings().Model(), converted, genConfig)

	session := &geminiStreamSession{
		thought:   "",
		metadata:  nil,
		tag:       randomUint64(),
		agentID:   params.Settings().ID(),
		startTime: time.Now(),
		g:         g,
	}

	return NewIterCloser(stream, session.Map, session.Collect(ctx, params.Settings().Model(), uint32(tokens.TotalTokens))), nil
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

func (s *geminiStreamSession) Collect(ctx context.Context, model string, wantInputTokens uint32) func(chatmodel.UsageStats, *genai.GenerateContentResponse) chatmodel.UsageStats {
	return func(u chatmodel.UsageStats, msg *genai.GenerateContentResponse) chatmodel.UsageStats {
		if msg.UsageMetadata != nil {
			u.InputTokens = uint32(max(0, msg.UsageMetadata.PromptTokenCount))
			u.OutputTokens = uint32(max(0, msg.UsageMetadata.CandidatesTokenCount))

			if u.InputTokens != wantInputTokens {
				s.g.log.TokenCountMismatch(ctx, model, wantInputTokens, u.InputTokens)
			}
		}

		u.Duration = time.Since(s.startTime)

		return u
	}
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
	PreflightCheck() chatmodel.PreflightFunc
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

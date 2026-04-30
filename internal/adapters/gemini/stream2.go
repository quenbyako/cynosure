package gemini

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/genai"

	"github.com/quenbyako/cynosure/internal/adapters/gemini/datatransfer"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/chatmodel"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

func (g *GeminiModel) StreamWithStats(
	ctx context.Context,
	input []messages.Message,
	settings entities.AgentReadOnly,
	opts ...chatmodel.StreamOption,
) (chatmodel.Iter, error) {
	if uint(len(input)) > g.hardCap {
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

	stream := g.client.Models.GenerateContentStream(ctx, params.Settings().Model(), converted, genConfig)

	session := &geminiStreamSession{
		thought:   "",
		metadata:  nil,
		tag:       randomUint64(),
		agentID:   params.Settings().ID(),
		startTime: time.Now(),
	}

	return NewIterCloser(stream, session.Map, session.Collect), nil
}

type geminiStreamSession struct {
	thought   string
	metadata  []byte
	tag       uint64
	agentID   ids.AgentID
	startTime time.Time
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

func (s *geminiStreamSession) Collect(u chatmodel.UsageStats, msg *genai.GenerateContentResponse) chatmodel.UsageStats {
	if msg.UsageMetadata != nil {
		u.InputTokens = uint32(max(0, msg.UsageMetadata.PromptTokenCount))
		u.OutputTokens = uint32(max(0, msg.UsageMetadata.CandidatesTokenCount))
	}

	u.Duration = time.Since(s.startTime)

	return u
}

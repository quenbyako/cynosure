package chat

import (
	"context"
	"errors"
	"fmt"
	"iter"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/aggregates/chat"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/messages"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/tools"
)

type GenerateResponseOpt func(*generateResponseParams)

type generateResponseParams struct {
	toolChoice tools.ToolChoice
	model      ids.ModelConfigID
}

func WithToolChoice(toolChoice tools.ToolChoice) GenerateResponseOpt {
	return func(params *generateResponseParams) { params.toolChoice = toolChoice }
}

func (s *Service) GenerateResponse(ctx context.Context, user ids.UserID, threadID string, msg messages.MessageUser, opts ...GenerateResponseOpt) (iter.Seq2[messages.Message, error], error) {
	ctx, span := s.trace.Start(ctx, "Service.GenerateResponse")
	defer span.End()

	p := generateResponseParams{
		toolChoice: tools.ToolChoiceForbidden,
		model:      s.defaultModel,
	}
	for _, opt := range opts {
		opt(&p)
	}

	modelConfig, err := s.models.GetModel(ctx, p.model)
	if err != nil {
		return nil, fmt.Errorf("getting model: %w", err)
	}

	c, err := s.getOrCreateChat(ctx, user, threadID, msg)
	if err != nil {
		return nil, err
	}

	return s.runConversationLoop(ctx, c, modelConfig, p.toolChoice), nil
}

// getOrCreateChat загружает существующий чат или создает новый, добавляя в него первое сообщение.
func (s *Service) getOrCreateChat(ctx context.Context, user ids.UserID, threadID string, initialMsg messages.MessageUser) (*chat.Chat, error) {
	c, err := chat.New(ctx, s.storage, s.tools, s.accounts, user, threadID)
	if errors.Is(err, ports.ErrNotFound) {
		if c, err = chat.CreateChatAggregate(ctx, s.storage, s.tools, s.accounts, user, threadID, nil); err != nil {
			return nil, fmt.Errorf("creating chat: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("loading chat: %w", err)
	}

	if err = c.AddMessage(ctx, initialMsg); err != nil {
		return nil, fmt.Errorf("adding user message: %w", err)
	}

	return c, nil
}

func (s *Service) runConversationLoop(ctx context.Context, c *chat.Chat, config entities.ModelSettingsReadOnly, toolChoice tools.ToolChoice) iter.Seq2[messages.Message, error] {
	return func(yield func(messages.Message, error) bool) {
		const maxTurns = 10
		for i := 0; i < maxTurns; i++ {
			toolRequests, continueLoop, err := s.processModelStream(ctx, c, config, toolChoice, yield)
			if err != nil {
				yield(nil, err)
				return
			}
			if !continueLoop {
				return // Завершаем, если не было вызовов инструментов
			}

			if len(toolRequests) > 0 {
				s.log.ToolCalled(ctx, c.ThreadID(), "some_user", toolRequests)
			}

			if err := s.executeToolCalls(ctx, c, toolRequests, yield); err != nil {
				yield(nil, err)
				return
			}

			if i == maxTurns-1 {
				s.log.MaxTurnsReached(ctx, c.ThreadID(), "some user id, i didn't pass it here yet")
			}
		}
	}
}

// processModelStream обрабатывает поток сообщений от модели.
func (s *Service) processModelStream(ctx context.Context, c *chat.Chat, config entities.ModelSettingsReadOnly, toolChoice tools.ToolChoice, yield func(messages.Message, error) bool) ([]messages.MessageToolRequest, bool, error) {
	var streamOpts []ports.StreamOption
	if toolChoice != tools.ToolChoiceForbidden {
		streamOpts = append(streamOpts, ports.WithStreamTools(c.RelevantTools()))
	}

	msgsStream, err := s.model.Stream(ctx, c.Messages(), config, streamOpts...)
	if err != nil {
		return nil, false, fmt.Errorf("starting model stream: %w", err)
	}

	var toolRequests []messages.MessageToolRequest
	mergedStream := messages.MergeMessagesStreaming(msgsStream)

	for msg, err := range mergedStream {
		if err != nil {
			return nil, false, fmt.Errorf("streaming messages: %w", err)
		}
		if err := c.AddMessage(ctx, msg); err != nil {
			return nil, false, fmt.Errorf("adding model message: %w", err)
		}
		if !yield(msg, nil) {
			return nil, false, nil // Итерация прервана потребителем
		}
		if toolReq, ok := msg.(messages.MessageToolRequest); ok {
			toolRequests = append(toolRequests, toolReq)
		}
	}

	return toolRequests, len(toolRequests) > 0, nil
}

// executeToolCalls выполняет вызовы инструментов и передает результаты.
func (s *Service) executeToolCalls(ctx context.Context, c *chat.Chat, toolRequests []messages.MessageToolRequest, yield func(messages.Message, error) bool) error {
	for _, toolReq := range toolRequests {
		call, err := c.DecodeToolCall(toolReq.ToolName(), toolReq.Arguments())
		if err != nil {
			return fmt.Errorf("decoding tool call: %w", err)
		}

		toolResultMsg, err := s.tools.ExecuteTool(ctx, call)
		if err != nil {
			return fmt.Errorf("executing tool: %w", err)
		}

		if err := c.AddMessage(ctx, toolResultMsg); err != nil {
			return fmt.Errorf("adding tool result message: %w", err)
		}

		if !yield(toolResultMsg, nil) {
			return nil // Итерация прервана потребителем
		}
	}
	return nil
}

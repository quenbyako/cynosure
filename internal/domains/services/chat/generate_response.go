package chat

import (
	"context"
	"errors"
	"fmt"
	"iter"

	"tg-helper/internal/domains/aggregates/chat"
	"tg-helper/internal/domains/components/ids"
	"tg-helper/internal/domains/components/messages"
	"tg-helper/internal/domains/components/tools"
	"tg-helper/internal/domains/ports"
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

	c, err := chat.New(ctx, s.storage, s.tools, s.accounts, user, threadID)
	if err == nil {
		if err = c.AddMessage(ctx, msg); err != nil {
			return nil, err
		}
	} else if errors.Is(err, ports.ErrNotFound) {
		c, err = chat.CreateChatAggregate(ctx, s.storage, s.tools, s.accounts, user, threadID, []messages.Message{
			msg,
		})
	}
	if err != nil {
		return nil, err
	}

	var commitedMessages []messages.Message

	var hasToolCalls bool
	const maxTurns = 10
	return func(yield func(messages.Message, error) bool) {

		for range maxTurns {
			hasToolCalls = false

			var streamOpts []ports.StreamOption
			if p.toolChoice != tools.ToolChoiceForbidden {
				streamOpts = append(streamOpts, ports.WithStreamTools(c.RelevantTools()))
			}

			msgs, err := s.model.Stream(ctx, c.Messages(), modelConfig, streamOpts...)
			if err != nil {
				return commitedMessages, err
			}

			for msg, err := range messages.MergeMessagesStreaming(msgs) {
				if err != nil {
					return commitedMessages, fmt.Errorf("streaming messages: %w", err)
				}
				if err = c.AddMessage(ctx, msg); err != nil {
					return commitedMessages, err
				}

				commitedMessages = append(commitedMessages, msg)

				if msg, ok := msg.(messages.MessageToolRequest); ok {
					call, err := c.DecodeToolCall(msg.ToolName(), msg.Arguments())
					if err != nil {
						return commitedMessages, err
					}

					msg, err := s.tools.ExecuteTool(ctx, call)
					if err != nil {
						return commitedMessages, err
					}

					if err = c.AddMessage(ctx, msg); err != nil {
						return commitedMessages, err
					}

					hasToolCalls = true
				}
			}

			if !hasToolCalls {
				break
			}
		}

		if hasToolCalls {
			fmt.Println("Log for later: Model reached max turns with tool calls, consider adjusting settings.")
		}
	}, nil
}

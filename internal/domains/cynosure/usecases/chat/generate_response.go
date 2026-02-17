package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/aggregates/chat"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
)

type GenerateResponseOpt func(*generateResponseParams)

type generateResponseParams struct {
	toolChoice tools.ToolChoice
	model      ids.AgentID
}

func WithToolChoice(toolChoice tools.ToolChoice) GenerateResponseOpt {
	return func(params *generateResponseParams) { params.toolChoice = toolChoice }
}

func (s *Usecase) GenerateResponse(ctx context.Context, threadID ids.ThreadID, msg messages.MessageUser, opts ...GenerateResponseOpt) (iter.Seq2[messages.Message, error], error) {
	ctx, span := s.trace.Start(ctx, "Service.GenerateResponse")
	defer span.End()

	p := generateResponseParams{
		toolChoice: tools.ToolChoiceAllowed,
		model:      s.defaultModel,
	}
	for _, opt := range opts {
		opt(&p)
	}

	modelConfig, err := s.models.GetAgent(ctx, p.model)
	if err != nil {
		return nil, fmt.Errorf("getting model: %w", err)
	}

	c, err := s.loadOrCreateChat(ctx, threadID, msg)
	if err != nil {
		return nil, err
	}

	return s.agentLoop(ctx, c, modelConfig, p.toolChoice), nil
}

// loadOrCreateChat retrieves an existing chat session by its thread ID or
// creates a new one if it doesn't exist. It then appends the incoming user
// message to the chat history.
func (s *Usecase) loadOrCreateChat(ctx context.Context, threadID ids.ThreadID, msg messages.MessageUser) (*chat.Chat, error) {
	c, err := chat.New(ctx, s.storage, s.indexer, s.toolStorage, s.accounts, s.models, threadID)
	if errors.Is(err, ports.ErrNotFound) {
		c, err = chat.CreateChatAggregate(ctx, s.storage, s.indexer, s.toolStorage, s.accounts, s.models, threadID, []messages.Message{msg})
		if err != nil {
			return nil, fmt.Errorf("creating chat: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("loading chat: %w", err)
	} else {
		if err = c.AcceptUserMessage(ctx, msg); err != nil {
			return nil, fmt.Errorf("adding user message: %w", err)
		}
	}

	return c, nil
}

func (s *Usecase) agentLoop(ctx context.Context, c *chat.Chat, config entities.AgentReadOnly, toolChoice tools.ToolChoice) iter.Seq2[messages.Message, error] {
	return func(yield func(messages.Message, error) bool) {
		ctx, span := s.trace.Start(ctx, "Usecase.agentLoop")
		defer span.End()

		for turn := range s.agentLoopTurns {
			span.AddEvent("set.turn", trace.WithAttributes(
				attribute.Int("turn", int(turn)),
			))

			toolRequests, shouldContinue := s.askModel(ctx, c, config, toolChoice, yield)
			if !shouldContinue {
				return
			}

			if len(toolRequests) == 0 {
				return
			}

			s.log.ToolCalled(ctx, c.ThreadID(), toolRequests)

			if !s.executeTools(ctx, c, toolRequests, yield) {
				return
			}

			if turn == s.agentLoopTurns-1 {
				s.log.MaxTurnsReached(ctx, c.ThreadID())
			}
		}
	}
}

func (s *Usecase) askModel(ctx context.Context, c *chat.Chat, config entities.AgentReadOnly, toolChoice tools.ToolChoice, yield func(messages.Message, error) bool) ([]messages.MessageToolRequest, bool) {
	stream, err := s.callModel(ctx, c, config, toolChoice)
	if err != nil {
		return nil, s.handleModelError(ctx, c, err, yield)
	}

	return s.streamModelMessages(ctx, c, stream, yield)
}

func (s *Usecase) callModel(ctx context.Context, c *chat.Chat, config entities.AgentReadOnly, toolChoice tools.ToolChoice) (iter.Seq2[messages.Message, error], error) {
	var opts []ports.StreamOption
	if toolChoice != tools.ToolChoiceForbidden {
		opts = append(opts, ports.WithStreamToolbox(c.RelevantTools()))
	}
	return s.model.Stream(ctx, c.Messages(), config, opts...)
}

func (s *Usecase) handleModelError(ctx context.Context, c *chat.Chat, err error, yield func(messages.Message, error) bool) bool {
	errorMsg, msgErr := messages.NewMessageAssistant(
		fmt.Sprintf("I apologize, but I encountered a technical error while processing your request: %v", err),
	)
	if msgErr != nil {
		yield(nil, fmt.Errorf("creating error message: %w", msgErr))
		return false
	}

	if acceptErr := c.AcceptAssistantMessage(ctx, errorMsg); acceptErr != nil {
		yield(nil, fmt.Errorf("saving error message: %w", acceptErr))
		return false
	}

	yield(errorMsg, err)
	return false
}

func (s *Usecase) streamModelMessages(ctx context.Context, c *chat.Chat, stream iter.Seq2[messages.Message, error], yield func(messages.Message, error) bool) ([]messages.MessageToolRequest, bool) {
	var toolRequests []messages.MessageToolRequest

	for msg, err := range messages.MergeMessagesStreaming(stream) {
		if err != nil {
			yield(nil, fmt.Errorf("streaming messages: %w", err))
			return nil, false
		}

		if !s.saveAndYieldMessage(ctx, c, msg, &toolRequests, yield) {
			return nil, false
		}
	}

	return toolRequests, true
}

func (s *Usecase) saveAndYieldMessage(ctx context.Context, c *chat.Chat, msg messages.Message, toolRequests *[]messages.MessageToolRequest, yield func(messages.Message, error) bool) bool {
	var err error
	switch v := msg.(type) {
	case messages.MessageAssistant:
		err = c.AcceptAssistantMessage(ctx, v)
	case messages.MessageToolRequest:
		err = c.AcceptToolRequest(ctx, v)
		*toolRequests = append(*toolRequests, v)
	default:
		err = fmt.Errorf("unexpected message type: %T", msg)
	}

	if err != nil {
		yield(nil, fmt.Errorf("saving message: %w", err))
		return false
	}

	return yield(msg, nil)
}

func (s *Usecase) executeTools(ctx context.Context, c *chat.Chat, toolRequests []messages.MessageToolRequest, yield func(messages.Message, error) bool) bool {
	for _, req := range toolRequests {
		if !s.executeTool(ctx, c, req, yield) {
			return false
		}
	}
	return true
}

func (s *Usecase) executeTool(ctx context.Context, c *chat.Chat, req messages.MessageToolRequest, yield func(messages.Message, error) bool) bool {
	toolID, cleanArgs, err := c.RelevantTools().ConvertRequest(req.ToolName(), req.Arguments())
	if err != nil {
		return yieldToolError(ctx, c, req, fmt.Sprintf("Failed to resolve tool: %v", err), yield)
	}

	tool, err := s.toolStorage.GetTool(ctx, toolID.Account(), toolID)
	if err != nil {
		return yieldToolError(ctx, c, req, fmt.Sprintf("Tool not found: %v", err), yield)
	}

	result, err := s.tools.ExecuteTool(ctx, *tool, cleanArgs, req.ToolCallID())
	if err != nil {
		return yieldToolError(ctx, c, req, fmt.Sprintf("Execution failed: %v", err), yield)
	}

	if err := c.AcceptToolResult(ctx, result); err != nil {
		yield(nil, fmt.Errorf("saving tool result: %w", err))
		return false
	}

	return yield(result, nil)
}

func yieldToolError(ctx context.Context, c *chat.Chat, req messages.MessageToolRequest, errMsg string, yield func(messages.Message, error) bool) bool {
	content, _ := json.Marshal(map[string]string{"error": errMsg})
	toolErr, err := messages.NewMessageToolError(
		content,
		req.ToolName(),
		req.ToolCallID(),
	)
	if err != nil {
		yield(nil, fmt.Errorf("building tool error object: %w", err))
		return false
	}

	if err = c.AcceptToolResult(ctx, toolErr); err != nil {
		yield(nil, fmt.Errorf("saving tool error: %w", err))
		return false
	}

	return yield(toolErr, nil)
}

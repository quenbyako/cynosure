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
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/chatmodel"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/ratelimiter"
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

func (u *Usecase) GenerateResponse(
	ctx context.Context,
	threadID ids.ThreadID,
	msg messages.MessageUser,
	opts ...GenerateResponseOpt,
) (iter.Seq2[messages.Message, error], error) {
	ctx, span := u.trace.Start(ctx, "Service.GenerateResponse")
	defer span.End()

	params := u.resolveGenerateResponseParams(opts)

	allow, err := u.allowLimits(ctx, threadID.User())
	if err != nil {
		return nil, err
	} else if !allow {
		return u.yieldRateLimitError(err)
	}

	chatAgg, modelConfig, err := u.getAgentWithChat(ctx, threadID, params.model, msg)
	if err != nil {
		return nil, fmt.Errorf("getting model: %w", err)
	}

	return u.agentLoop(ctx, chatAgg, modelConfig, params.toolChoice), nil
}

func (u *Usecase) allowLimits(ctx context.Context, id ids.UserID) (bool, error) {
	err := u.limiter.Consume(ctx, id, 1)
	if err == nil {
		return true, nil
	}

	if errors.Is(err, ratelimiter.ErrRateLimitExceeded) {
		return false, nil
	}

	return false, fmt.Errorf("checking rate limit: %w", err)
}

func (u *Usecase) getAgentWithChat(
	ctx context.Context, thread ids.ThreadID, agent ids.AgentID, msg messages.MessageUser,
) (*chat.Chat, entities.AgentReadOnly, error) {
	chatAgg, err := u.loadOrCreateChat(ctx, thread, msg)
	if err != nil {
		return nil, nil, err
	}

	agentID, err := u.resolveAgentID(ctx, thread, chatAgg, agent)
	if err != nil {
		return nil, nil, err
	}

	modelConfig, err := u.agents.GetAgent(ctx, agentID)
	if err != nil {
		return nil, nil, fmt.Errorf("getting model: %w", err)
	}

	return chatAgg, modelConfig, nil
}

func (u *Usecase) resolveGenerateResponseParams(opts []GenerateResponseOpt) generateResponseParams {
	params := generateResponseParams{
		toolChoice: tools.ToolChoiceAllowed,
		model:      ids.AgentID{},
	}
	for _, opt := range opts {
		opt(&params)
	}

	return params
}

func (u *Usecase) resolveAgentID(
	ctx context.Context,
	threadID ids.ThreadID,
	chatAgg *chat.Chat,
	requestedAgent ids.AgentID,
) (ids.AgentID, error) {
	if requestedAgent.Valid() {
		return requestedAgent, nil
	}

	agentID := chatAgg.AgentID()
	if agentID.Valid() {
		return agentID, nil
	}

	return u.deduceAgentID(ctx, threadID, chatAgg)
}

func (u *Usecase) deduceAgentID(
	ctx context.Context,
	threadID ids.ThreadID,
	chatAgg *chat.Chat,
) (ids.AgentID, error) {
	agents, err := u.agents.ListAgents(ctx, threadID.User())
	if err != nil {
		return ids.AgentID{}, fmt.Errorf("listing user agents: %w", err)
	}

	if len(agents) == 0 {
		return ids.AgentID{}, fmt.Errorf("listing user agents: %w", ErrNoAgentsFound)
	}

	// TODO: need to select agent. For now, just take the first one.
	agentID := agents[0].ID()
	if err := chatAgg.SetAgent(ctx, agentID); err != nil {
		return ids.AgentID{}, fmt.Errorf("setting default agent to thread: %w", err)
	}

	return agentID, nil
}

// loadOrCreateChat retrieves an existing chat session by its thread ID or
// creates a new one if it doesn't exist. It then appends the incoming user
// message to the chat history.
func (u *Usecase) loadOrCreateChat(
	ctx context.Context,
	id ids.ThreadID,
	msg messages.MessageUser,
) (*chat.Chat, error) {
	agg, err := chat.New(ctx,
		u.storage, u.indexer, u.toolStorage, u.accounts,
		id, u.defaultChatLimit,
	)
	switch {
	case errors.Is(err, ports.ErrNotFound):
		agg, err = chat.CreateChatAggregate(
			ctx, u.storage, u.indexer, u.toolStorage, u.accounts,
			id, []messages.Message{msg},
			u.defaultChatLimit,
		)
		if err != nil {
			return nil, fmt.Errorf("creating chat: %w", err)
		}
	case err != nil:
		return nil, fmt.Errorf("loading chat: %w", err)
	default:
		if err = agg.AcceptUserMessage(ctx, msg); err != nil {
			return nil, fmt.Errorf("adding user message: %w", err)
		}
	}

	return agg, nil
}

func (u *Usecase) agentLoop(
	ctx context.Context,
	thread *chat.Chat,
	config entities.AgentReadOnly,
	toolChoice tools.ToolChoice,
) iter.Seq2[messages.Message, error] {
	return func(yield func(messages.Message, error) bool) {
		loopCtx, span := u.trace.Start(ctx, "Usecase.agentLoop")
		defer span.End()

		for turn := range u.agentLoopTurns {
			if !u.agentTurn(loopCtx, thread, config, toolChoice, turn, yield) {
				return
			}
		}
	}
}

func (u *Usecase) agentTurn(
	ctx context.Context,
	thread *chat.Chat,
	config entities.AgentReadOnly,
	toolChoice tools.ToolChoice,
	turn uint8,
	yield func(messages.Message, error) bool,
) bool {
	span := trace.SpanFromContext(ctx)
	span.AddEvent("set.turn", trace.WithAttributes(attribute.Int("turn", int(turn))))

	toolRequests, shouldContinue := u.askModel(ctx, thread, config, toolChoice, yield)
	if !shouldContinue || len(toolRequests) == 0 {
		return false
	}

	if !u.handleToolRequests(ctx, thread, turn, toolRequests, yield) {
		return false
	}

	return true
}

func (u *Usecase) handleToolRequests(
	ctx context.Context,
	thread *chat.Chat,
	turn uint8,
	toolRequests []messages.MessageToolRequest,
	yield func(messages.Message, error) bool,
) bool {
	u.log.ToolCalled(ctx, thread.ThreadID().String(), toolRequests)

	if !u.executeTools(ctx, thread, toolRequests, yield) {
		return false
	}

	if turn == u.agentLoopTurns-1 {
		u.log.MaxTurnsReached(ctx, thread.ThreadID().String())
	}

	return true
}

func (u *Usecase) askModel(
	ctx context.Context,
	thread *chat.Chat,
	config entities.AgentReadOnly,
	toolChoice tools.ToolChoice,
	yield func(messages.Message, error) bool,
) ([]messages.MessageToolRequest, bool) {
	stream, err := u.callModel(ctx, thread, config, toolChoice)
	if err != nil {
		u.handleModelError(ctx, thread, err, yield)
		return nil, false
	}

	return u.streamModelMessages(ctx, thread, stream, yield)
}

func (u *Usecase) callModel(
	ctx context.Context,
	thread *chat.Chat,
	config entities.AgentReadOnly,
	toolChoice tools.ToolChoice,
) (iter.Seq2[messages.Message, error], error) {
	var opts []chatmodel.StreamOption
	if toolChoice != tools.ToolChoiceForbidden {
		opts = append(opts, chatmodel.WithStreamToolbox(thread.RelevantTools()))
	}

	maxContext, ok := config.MaxContext()
	if !ok {
		maxContext = u.defaultChatLimit
	}

	resp, err := u.model.Stream(ctx, thread.Messages(maxContext), config, opts...)
	if err != nil {
		return nil, fmt.Errorf("calling model stream: %w", err)
	}

	return resp, nil
}

func (u *Usecase) handleModelError(
	ctx context.Context,
	thread *chat.Chat,
	err error,
	yield func(messages.Message, error) bool,
) {
	errorMsg, msgErr := messages.NewMessageAssistant(
		fmt.Sprintf(
			"I apologize, but I encountered a technical error while processing your request: %v",
			err,
		),
	)
	if msgErr != nil {
		yield(nil, fmt.Errorf("creating error message: %w", msgErr))
		return
	}

	if acceptErr := thread.AcceptAssistantMessage(ctx, errorMsg); acceptErr != nil {
		yield(nil, fmt.Errorf("saving error message: %w", acceptErr))
		return
	}

	yield(errorMsg, err)
}

func (u *Usecase) streamModelMessages(
	ctx context.Context,
	thread *chat.Chat,
	stream iter.Seq2[messages.Message, error],
	yield func(messages.Message, error) bool,
) ([]messages.MessageToolRequest, bool) {
	var toolRequests []messages.MessageToolRequest

	for msg, err := range messages.MergeMessagesStreaming(stream) {
		if err != nil {
			yield(nil, fmt.Errorf("streaming messages: %w", err))
			return nil, false
		}

		if !u.saveAndYieldMessage(ctx, thread, msg, &toolRequests, yield) {
			return nil, false
		}
	}

	return toolRequests, true
}

func (u *Usecase) saveAndYieldMessage(
	ctx context.Context,
	thread *chat.Chat,
	msg messages.Message,
	toolRequests *[]messages.MessageToolRequest,
	yield func(messages.Message, error) bool,
) bool {
	var err error

	switch v := msg.(type) {
	case messages.MessageAssistant:
		err = thread.AcceptAssistantMessage(ctx, v)
	case messages.MessageToolRequest:
		err = thread.AcceptToolRequest(ctx, v)
		*toolRequests = append(*toolRequests, v)
	default:
		err = errInternalValidation("unexpected message type: %T", msg)
	}

	if err != nil {
		yield(nil, fmt.Errorf("saving message: %w", err))
		return false
	}

	return yield(msg, nil)
}

func (u *Usecase) executeTools(
	ctx context.Context,
	c *chat.Chat,
	toolRequests []messages.MessageToolRequest,
	yield func(messages.Message, error) bool,
) bool {
	for _, req := range toolRequests {
		if !u.executeTool(ctx, c, req, yield) {
			return false
		}
	}

	return true
}

func (u *Usecase) executeTool(
	ctx context.Context,
	thread *chat.Chat,
	req messages.MessageToolRequest,
	yield func(messages.Message, error) bool,
) bool {
	toolID, cleanArgs, err := thread.RelevantTools().ConvertRequest(req.ToolName(), req.Arguments())
	if err != nil {
		return yieldToolError(
			ctx, thread, req, fmt.Sprintf("Failed to resolve tool: %v", err), yield,
		)
	}

	tool, err := u.toolStorage.GetTool(ctx, toolID.Account(), toolID)
	if err != nil {
		return yieldToolError(ctx, thread, req, fmt.Sprintf("Tool not found: %v", err), yield)
	}

	result, err := u.tools.ExecuteTool(ctx, tool, cleanArgs, req.ToolCallID())
	if err != nil {
		return yieldToolError(ctx, thread, req, fmt.Sprintf("Execution failed: %v", err), yield)
	}

	if err := thread.AcceptToolResult(ctx, result); err != nil {
		yield(nil, fmt.Errorf("saving tool result: %w", err))
		return false
	}

	return yield(result, nil)
}

func yieldToolError(
	ctx context.Context,
	thread *chat.Chat,
	req messages.MessageToolRequest,
	errMsg string,
	yield func(messages.Message, error) bool,
) bool {
	content, err := json.Marshal(map[string]string{"error": errMsg})
	if err != nil {
		yield(nil, fmt.Errorf("building tool error json: %w", err))
		return false
	}

	toolErr, err := messages.NewMessageToolError(
		content,
		req.ToolName(),
		req.ToolCallID(),
	)
	if err != nil {
		yield(nil, fmt.Errorf("building tool error object: %w", err))
		return false
	}

	if err = thread.AcceptToolResult(ctx, toolErr); err != nil {
		yield(nil, fmt.Errorf("saving tool error: %w", err))
		return false
	}

	return yield(toolErr, nil)
}

func (u *Usecase) yieldRateLimitError(err error) (iter.Seq2[messages.Message, error], error) {
	return func(yield func(messages.Message, error) bool) {
		errorMsg, msgErr := messages.NewMessageAssistant(
			"I apologize, but you have reached your message rate limit. " +
				"Please wait a moment before sending another message.",
		)
		if msgErr != nil {
			yield(nil, fmt.Errorf("creating rate limit error message: %w", msgErr))
			return
		}

		yield(errorMsg, nil)
	}, nil
}

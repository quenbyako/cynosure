package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/aggregates/chat"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/chatmodel"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/embedding"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/ratelimiter"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
)

var errSettleFuncNotSet = errors.New("settle func wasn't set")

func defaultGenerateResponseParams(required generateResponseRequiredParams) generateResponseParams {
	return generateResponseParams{
		generateResponseRequiredParams: required,
		toolChoice:                     tools.ToolChoiceAllowed,
		model:                          ids.AgentID{},
	}
}

func (p generateResponseParams) validate() error {
	switch {
	case !p.threadID.Valid():
		return errInternalValidation("thread id is required")
	case !p.msg.Valid():
		return errInternalValidation("message is required")
	default:
		return nil
	}
}

// GenerateResponse creates or loads a chat session and generates a response
// from the model.
//
// Throws:
//
//   - [RateLimitExceededError]
func (u *Usecase) GenerateResponse(
	ctx context.Context,
	threadID ids.ThreadID,
	msg messages.MessageUser,
	opts ...GenerateResponseOption,
) (iter.Seq2[messages.Message, error], error) {
	ctx, span := u.obs.generateResponse(ctx)

	params, err := buildGenerateResponseParams(threadID, msg, opts...)
	if err != nil {
		span.recordError(err)
		span.end()

		return nil, err
	}

	chatAgg, modelConfig, err := u.getAgentWithChat(ctx, threadID, params.model, msg)
	if err != nil {
		if e := new(ratelimiter.RateLimitExceededError); errors.As(err, &e) {
			err = ErrRateLimitExceeded(e.RetryAt())
		}

		span.recordError(err)
		span.end()

		return nil, err
	}

	return u.agentLoop(ctx, span, chatAgg, modelConfig, params.toolChoice), nil
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
	agg, err := u.loadChat(ctx, id)
	if errors.Is(err, ports.ErrNotFound) {
		return u.createChat(ctx, id, msg)
	}

	if err != nil {
		return nil, err
	}

	if err := agg.AcceptUserMessage(ctx, msg); err != nil {
		if e := new(ratelimiter.RateLimitExceededError); errors.As(err, &e) {
			return nil, ErrRateLimitExceeded(e.RetryAt())
		}

		return nil, fmt.Errorf("adding user message: %w", err)
	}

	return agg, nil
}

func (u *Usecase) loadChat(ctx context.Context, id ids.ThreadID) (*chat.Chat, error) {
	agg, err := chat.New(ctx,
		u.storage, u.provideEmbeddings, u.toolStorage, u.accounts,
		id, u.defaultChatLimit,
	)
	if err == nil {
		return agg, nil
	}

	if e := new(ratelimiter.RateLimitExceededError); errors.As(err, &e) {
		return nil, ErrRateLimitExceeded(e.RetryAt())
	}

	return nil, fmt.Errorf("loading chat: %w", err)
}

func (u *Usecase) createChat(
	ctx context.Context, id ids.ThreadID, msg messages.MessageUser,
) (*chat.Chat, error) {
	agg, err := chat.CreateChatAggregate(
		ctx, u.storage,
		u.provideEmbeddings,
		u.toolStorage,
		u.accounts,
		id,
		[]messages.Message{msg},
		u.defaultChatLimit,
	)
	if err != nil {
		return nil, fmt.Errorf("creating chat: %w", err)
	}

	return agg, nil
}

// agentLoop executes the agent loop for a given chat and model configuration.
// It returns a sequence of messages and errors.
func (u *Usecase) agentLoop(
	ctx context.Context,
	span generateResponseCallback,
	thread *chat.Chat,
	config entities.AgentReadOnly,
	toolChoice tools.ToolChoice,
) iter.Seq2[messages.Message, error] {
	return func(yield func(messages.Message, error) bool) {
		defer span.end()

		usage := u.executeAgentLoop(ctx, thread, config, toolChoice, yield)

		span.recordTotalUsage(usage.InputTokens, usage.OutputTokens)
	}
}

func (u *Usecase) executeAgentLoop(
	ctx context.Context,
	thread *chat.Chat,
	config entities.AgentReadOnly,
	toolChoice tools.ToolChoice,
	yield func(messages.Message, error) bool,
) (usage chatmodel.UsageStats) {
	totalUsage := chatmodel.UsageStats{}

	for turn := range u.agentLoopTurns {
		usage, next := u.executeAgentTurn(
			ctx, thread, config, toolChoice, turn, yield,
		)

		totalUsage = u.accumulateUsage(totalUsage, usage)

		if !next {
			break
		}
	}

	return totalUsage
}

func (u *Usecase) executeAgentTurn(
	ctx context.Context,
	thread *chat.Chat,
	config entities.AgentReadOnly,
	toolChoice tools.ToolChoice,
	turn uint8,
	yield func(messages.Message, error) bool,
) (chatmodel.UsageStats, bool) {
	ctx, span := u.obs.agentLoopTurn(ctx, int(turn))
	defer span.end()

	settle := func(context.Context, int) error { return errSettleFuncNotSet }

	preflight := u.newPreflight(thread, &settle)

	toolRequests, usage, cont := u.askModel(ctx, thread, config, toolChoice, preflight, yield)

	u.obs.recordUsage(ctx, config.Model(), usage.InputTokens, usage.OutputTokens, usage.Duration)

	if err := settle(ctx, int(usage.OutputTokens)); err != nil {
		span.recordError(err)
		yield(nil, err)

		return usage, false
	}

	next := cont && len(toolRequests) > 0 &&
		u.handleToolRequests(ctx, thread, turn, toolRequests, yield)

	return usage, next
}

func (u *Usecase) newPreflight(
	thread *chat.Chat, turnSettlement *func(context.Context, int) error,
) chatmodel.PreflightFunc {
	return func(ctx context.Context, modelName string, inputTokens int) error {
		// Currently we charge for the full context in each turn, as most providers
		// charge for the entire prompt. However, if prompt caching is enabled,
		// the actual cost may be lower. This logic should eventually be
		// delegated to the model adapter or handled via a more sophisticated
		// quota management system.
		report, err := u.limiter.ConsumeChatRequests(
			ctx, thread.ThreadID().User(), modelName, inputTokens,
		)
		if err != nil {
			return fmt.Errorf("requesting chat quota: %w", err)
		}

		*turnSettlement = report

		return nil
	}
}

func (u *Usecase) accumulateUsage(total, current chatmodel.UsageStats) chatmodel.UsageStats {
	return chatmodel.UsageStats{
		InputTokens:  current.InputTokens + total.InputTokens,
		OutputTokens: current.OutputTokens + total.OutputTokens,
		Duration:     current.Duration + total.Duration,
		CostUSD:      current.CostUSD.Add(total.CostUSD),
	}
}

func (u *Usecase) handleToolRequests(
	ctx context.Context,
	thread *chat.Chat,
	turn uint8,
	toolRequests []messages.MessageToolRequest,
	yield func(messages.Message, error) bool,
) bool {
	u.obs.toolCalled(ctx, thread.ThreadID().String(), toolRequests)

	if !u.executeTools(ctx, thread, toolRequests, yield) {
		return false
	}

	if turn == u.agentLoopTurns-1 {
		u.obs.maxTurnsReached(ctx, thread.ThreadID().String())
	}

	return true
}

func (u *Usecase) askModel(
	ctx context.Context,
	thread *chat.Chat,
	config entities.AgentReadOnly,
	toolChoice tools.ToolChoice,
	preflight chatmodel.PreflightFunc,
	yield func(messages.Message, error) bool,
) ([]messages.MessageToolRequest, chatmodel.UsageStats, bool) {
	stream, err := u.callModel(ctx, thread, config, toolChoice, preflight)
	if err != nil {
		yield(nil, fmt.Errorf("model stream error: %w", err))

		return nil, chatmodel.UsageStats{}, false
	}

	return u.streamModelMessages(ctx, thread, stream, yield)
}

func (u *Usecase) callModel(
	ctx context.Context,
	thread *chat.Chat,
	config entities.AgentReadOnly,
	toolChoice tools.ToolChoice,
	preflight chatmodel.PreflightFunc,
) (chatmodel.Iter, error) {
	opts := []chatmodel.StreamOption{chatmodel.WithPreflightCheck(preflight)}

	if toolChoice != tools.ToolChoiceForbidden {
		toolbox, err := thread.RelevantTools(ctx)
		if e := new(ratelimiter.RateLimitExceededError); errors.As(err, &e) {
			return nil, ErrRateLimitExceeded(e.RetryAt())
		}

		if err != nil {
			return nil, fmt.Errorf("getting relevant tools: %w", err)
		}

		opts = append(opts, chatmodel.WithStreamToolbox(toolbox))
	}

	maxContext, ok := config.MaxContext()
	if !ok {
		maxContext = u.defaultChatLimit
	}

	resp, err := u.model.StreamWithStats(ctx, thread.Messages(maxContext), config, opts...)
	if err == nil {
		return resp, nil
	}

	if e := new(ratelimiter.RateLimitExceededError); errors.As(err, &e) {
		return nil, ErrRateLimitExceeded(e.RetryAt())
	}

	return nil, fmt.Errorf("calling model stream: %w", err)
}

func (u *Usecase) streamModelMessages(
	ctx context.Context,
	thread *chat.Chat,
	iterator chatmodel.Iter,
	yield func(messages.Message, error) bool,
) (toolRequests []messages.MessageToolRequest, usage chatmodel.UsageStats, ok bool) {
	var err error

	defer func() {
		// Ensure iterator is closed and usage stats are captured even on early exit.
		// We ignore the error here because we are already in the process of returning,
		// but the usage stats are preserved in the named return variable.
		usage, err = iterator.Close()
		if err != nil {
			yield(nil, fmt.Errorf("closing iterator: %w", err))
		}
	}()

	// We wrap our pull iterator to a Seq2 to use our existing streaming merge logic.
	for msg, err := range messages.MergeMessagesStreaming(u.iteratorToSeq2(iterator)) {
		if err != nil {
			yield(nil, fmt.Errorf("streaming messages: %w", err))
			return nil, chatmodel.UsageStats{}, false
		}

		if !u.saveAndYieldMessage(ctx, thread, msg, &toolRequests, yield) {
			return nil, chatmodel.UsageStats{}, false
		}
	}

	return toolRequests, usage, true
}

func (u *Usecase) iteratorToSeq2(iterator chatmodel.Iter) iter.Seq2[messages.Message, error] {
	return func(yield func(messages.Message, error) bool) {
		for {
			msg, ok := iterator.Next()
			if !ok {
				return
			}

			if !yield(msg, nil) {
				return
			}
		}
	}
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
	chatObj *chat.Chat,
	toolRequests []messages.MessageToolRequest,
	yield func(messages.Message, error) bool,
) bool {
	for _, req := range toolRequests {
		if !u.executeTool(ctx, chatObj, req, yield) {
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
	toolbox, err := thread.RelevantTools(ctx)
	if err != nil {
		return yieldToolError(
			ctx, thread, req, fmt.Sprintf("Failed to load tools: %v", err), yield,
		)
	}

	toolID, cleanArgs, err := toolbox.ConvertRequest(req.ToolName(), req.Arguments())
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

func (u *Usecase) provideEmbeddings(
	ctx context.Context, user ids.UserID, msgs []messages.Message,
) ([1536]float32, error) {
	preflight := func(ctx context.Context, modelName string, tokens int) error {
		return u.limiter.ConsumeEmbeddingRequests(ctx, user, modelName, tokens)
	}

	vector, err := u.indexer.BuildToolEmbedding(ctx, msgs, embedding.WithPreflightCheck(preflight))
	if err != nil {
		return [1536]float32{}, fmt.Errorf("building embedding: %w", err)
	}

	return vector, nil
}

package openrouter

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/OpenRouterTeam/go-sdk/models/components"
	"github.com/OpenRouterTeam/go-sdk/optionalnullable"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/quenbyako/cynosure/internal/adapters/openrouter/datatransfer"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/chatmodel"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
)

func (o *Adapter) prepareChatRequest(
	ctx context.Context,
	settings entities.AgentReadOnly,
	preflight chatmodel.PreflightFunc,
	toolbox tools.Toolbox,
	toolChoice tools.ToolChoice,
	input []messages.Message,
) (*http.Request, int, error) {
	totalTokens, err := o.checkPreflightAndRateLimits(ctx, settings, preflight, toolbox, input)
	if err != nil {
		return nil, 0, err
	}

	req, err := o.buildChatRequest(settings, toolbox, toolChoice, input)
	if err != nil {
		return nil, 0, err
	}

	httpReq, err := o.buildHTTPRequest(ctx, &req)
	if err != nil {
		return nil, 0, err
	}

	return httpReq, totalTokens, nil
}

func (o *Adapter) executeHTTPRequest(httpReq *http.Request) (*http.Response, error) {
	resp, err := o.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openrouter request failed: %w", err)
	}

	if err := o.checkResponseStatus(resp); err != nil {
		return nil, err
	}

	return resp, nil
}

func (o *Adapter) createSession(
	ctx context.Context,
	settings entities.AgentReadOnly,
	resp *http.Response,
	totalTokens int,
) (*streamSession, error) {
	if totalTokens < 0 || int64(totalTokens) > math.MaxUint32 {
		return nil, fmt.Errorf("%w: %d", ErrTokenCountOutOfBounds, totalTokens)
	}

	return o.newStreamSession(ctx, settings, resp.Body, uint32(totalTokens)), nil
}

func (o *Adapter) executeAndCreateSession(
	ctx context.Context,
	settings entities.AgentReadOnly,
	httpReq *http.Request,
	totalTokens int,
) (*streamSession, error) {
	resp, err := o.executeHTTPRequest(httpReq)
	if err != nil {
		return nil, err
	}

	session, err := o.createSession(ctx, settings, resp, totalTokens)
	if err != nil {
		_ = resp.Body.Close() //nolint:errcheck

		return nil, err
	}

	return session, nil
}

func (o *Adapter) StreamWithStats(
	ctx context.Context,
	input []messages.Message,
	settings entities.AgentReadOnly,
	opts ...chatmodel.StreamOption,
) (chatmodel.Iter, error) {
	if uint(len(input)) > o.maxMsgsPerReq {
		return nil, chatmodel.ErrHistoryTooLong
	}

	params, err := chatmodel.StreamParams(input, settings, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare stream params: %w", err)
	}

	httpReq, totalTokens, err := o.prepareChatRequest(
		ctx, settings, params.PreflightCheck(), params.Toolbox(), params.ToolChoice(), input,
	)
	if err != nil {
		return nil, err
	}

	return o.executeAndCreateSession(ctx, settings, httpReq, totalTokens)
}

func (o *Adapter) checkResponseStatus(resp *http.Response) error {
	if resp.StatusCode == http.StatusOK {
		return nil
	}

	defer resp.Body.Close() //nolint:errcheck

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read error body: %w", err)
	}

	return fmt.Errorf(
		"%w: status %d: %s",
		ErrRequestFailed,
		resp.StatusCode,
		string(bodyBytes),
	)
}

func (o *Adapter) buildHTTPRequest(
	ctx context.Context,
	chatReq *components.ChatRequest,
) (*http.Request, error) {
	reqBytes, err := json.Marshal(chatReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	const (
		url       = "https://openrouter.ai/api/v1/chat/completions"
		userAgent = "speakeasy-sdk/go 0.4.1 2.879.6 1.0.0 github.com/OpenRouterTeam/go-sdk"
	)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	httpReq.Header.Set("Accept", "application/json;q=1, text/event-stream;q=0")
	httpReq.Header.Set("User-Agent", userAgent)

	return httpReq, nil
}

func (o *Adapter) checkPreflightAndRateLimits(
	ctx context.Context,
	settings entities.AgentReadOnly,
	preflight chatmodel.PreflightFunc,
	toolbox tools.Toolbox,
	input []messages.Message,
) (int, error) {
	o.obs.streamStarted(ctx, settings.Model(), len(toolbox.List()))

	tokMsgs := datatransfer.ConvertMessagesForTokenCounting(settings.SystemMessage(), input)

	totalTokens, err := o.tokenCounter.CountTokens(ctx, settings.Model(), tokMsgs)
	if err != nil {
		totalTokens = o.tokenCounter.EstimateConservativeFallback(tokMsgs)
	}

	if err = preflight(ctx, settings.Model(), totalTokens); err != nil {
		return 0, err
	}

	if err = o.waitInputLimit(ctx, totalTokens); err != nil {
		return 0, err
	}

	return totalTokens, nil
}

func (o *Adapter) buildChatRequest(
	settings entities.AgentReadOnly,
	toolbox tools.Toolbox,
	toolChoice tools.ToolChoice,
	input []messages.Message,
) (components.ChatRequest, error) {
	openAIMsgs := datatransfer.ConvertMessages(settings.SystemMessage(), input)
	modelStr := settings.Model()
	streamBool := true

	//nolint:exhaustruct // too many fields
	req := components.ChatRequest{
		Model:    &modelStr,
		Messages: openAIMsgs,
		Stream:   &streamBool,
	}

	if temp, ok := settings.Temperature(); ok {
		tempVal := float64(temp)
		req.Temperature = optionalnullable.From(&tempVal)
	}

	if err := o.appendToolsToRequest(&req, toolbox, toolChoice); err != nil {
		return components.ChatRequest{}, err
	}

	return req, nil
}

func (o *Adapter) appendToolsToRequest(
	req *components.ChatRequest,
	toolbox tools.Toolbox,
	toolChoice tools.ToolChoice,
) error {
	toolList := toolbox.List()
	if len(toolList) == 0 {
		return nil
	}

	req.Tools = make([]components.ChatFunctionTool, len(toolList))
	for i, tool := range toolList {
		t, err := o.buildTool(tool)
		if err != nil {
			return fmt.Errorf("failed to build tool %q: %w", tool.Name(), err)
		}

		req.Tools[i] = t
	}

	choice, err := datatransfer.ConvertToolChoice(toolChoice)
	if err != nil {
		return fmt.Errorf("failed to convert tool choice: %w", err)
	}

	req.ToolChoice = &choice

	return nil
}

func (o *Adapter) buildTool(tool tools.RawTool) (components.ChatFunctionTool, error) {
	var paramsMap map[string]any

	if err := json.Unmarshal(tool.ConvertedSchema(), &paramsMap); err != nil {
		return components.ChatFunctionTool{}, fmt.Errorf("failed to unmarshal tool schema: %w", err)
	}

	desc := tool.Desc()

	return components.CreateChatFunctionToolChatFunctionToolFunction(
		components.ChatFunctionToolFunction{
			CacheControl: nil,
			Type:         components.ChatFunctionToolTypeFunction,
			Function: components.ChatFunctionToolFunctionFunction{
				Description: &desc,
				Name:        tool.Name(),
				Parameters:  paramsMap,
				Strict:      nil,
			},
		},
	), nil
}

func (o *Adapter) newStreamSession(
	ctx context.Context,
	settings entities.AgentReadOnly,
	body io.ReadCloser,
	wantInputTokens uint32,
) *streamSession {
	return &streamSession{
		ctx:             context.WithoutCancel(ctx),
		startTime:       time.Now(),
		adapter:         o,
		body:            body,
		scanner:         bufio.NewScanner(body),
		err:             nil,
		toolCalls:       make(map[int64]*toolCallAccumulator),
		pendingMessages: nil,
		model:           settings.Model(),
		mu:              sync.Mutex{},
		wantInputTokens: wantInputTokens,
		inputTokens:     0,
		outputTokens:    0,
		totalPrice:      decimal.Decimal{},
		agentID:         settings.ID(),
		yieldedTools:    false,
		finished:        false,
	}
}

func (o *Adapter) waitInputLimit(ctx context.Context, numTokens int) error {
	cause := chatmodel.ErrHardQuotaExhausted

	limCtx, cancel := context.WithTimeoutCause(ctx, maxLimiterWait, cause)
	defer cancel()

	if err := o.chatInputLimiter.WaitN(limCtx, numTokens); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(context.Cause(limCtx), context.Canceled) {
			return fmt.Errorf("chat rate limit exceeded: %w", err)
		}

		return chatmodel.ErrHardQuotaExhausted
	}

	return nil
}

type toolCallAccumulator struct {
	id        string
	name      string
	arguments strings.Builder
}

type openRouterChatChunkUsage struct {
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	Cost             float64 `json:"cost"`
}

type openRouterToolCall struct {
	ID       *string `json:"id"`
	Type     *string `json:"type"`
	Function *struct {
		Name      *string `json:"name"`
		Arguments *string `json:"arguments"`
	} `json:"function"`
	Index int64 `json:"index"`
}

type openRouterChatChunkChoiceDelta struct {
	Content   *string              `json:"content"`
	Reasoning *string              `json:"reasoning"`
	ToolCalls []openRouterToolCall `json:"tool_calls"`
}

type openRouterChatChunkChoice struct {
	Delta openRouterChatChunkChoiceDelta `json:"delta"`
}

type openRouterChatChunk struct {
	Usage   *openRouterChatChunkUsage   `json:"usage"`
	ID      string                      `json:"id"`
	Choices []openRouterChatChunkChoice `json:"choices"`
}

type streamSession struct {
	startTime       time.Time
	ctx             interface{ Value(key any) any }
	body            io.ReadCloser
	err             error
	adapter         *Adapter
	scanner         *bufio.Scanner
	toolCalls       map[int64]*toolCallAccumulator
	model           string
	totalPrice      decimal.Decimal
	pendingMessages []messages.Message
	mu              sync.Mutex
	wantInputTokens uint32
	inputTokens     uint32
	outputTokens    uint32
	agentID         ids.AgentID
	yieldedTools    bool
	finished        bool
}

var _ chatmodel.Iter = (*streamSession)(nil)

func (s *streamSession) popPendingMessage() (messages.Message, bool) {
	if len(s.pendingMessages) > 0 {
		msg := s.pendingMessages[0]
		s.pendingMessages = s.pendingMessages[1:]

		return msg, true
	}

	return nil, false
}

func (s *streamSession) scanAndYield() (messages.Message, bool) {
	msg, ok, err := s.scanNextLine()
	if err != nil {
		s.err = err
		return nil, false
	}

	if ok {
		return msg, true
	}

	if err := s.scanner.Err(); err != nil {
		s.err = fmt.Errorf("scanner error: %w", err)
		return nil, false
	}

	return s.yieldToolCalls()
}

func (s *streamSession) Next() (messages.Message, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.finished || s.err != nil {
		return nil, false
	}

	if msg, ok := s.popPendingMessage(); ok {
		return msg, true
	}

	if s.yieldedTools {
		return nil, false
	}

	return s.scanAndYield()
}

func (s *streamSession) scanNextLine() (messages.Message, bool, error) {
	for s.scanner.Scan() {
		chunk, ok, err := s.parseLine(s.scanner.Text())
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return nil, false, err
		}

		if !ok {
			continue
		}

		if msg, ok := s.processStreamChunk(chunk); ok {
			return msg, true, nil
		}
	}

	return nil, false, nil
}

func (s *streamSession) parseLine(line string) (openRouterChatChunk, bool, error) {
	if line == "" || strings.HasPrefix(line, ":") || !strings.HasPrefix(line, "data: ") {
		return openRouterChatChunk{}, false, nil
	}

	data := strings.TrimPrefix(line, "data: ")
	if data == "[DONE]" {
		return openRouterChatChunk{}, false, io.EOF
	}

	var chunk openRouterChatChunk
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return openRouterChatChunk{}, false, fmt.Errorf("failed to unmarshal chunk: %w", err)
	}

	return chunk, true, nil
}

func (s *streamSession) processStreamChunk(chunk openRouterChatChunk) (messages.Message, bool) {
	s.parseUsage(chunk.Usage)

	if len(chunk.Choices) == 0 {
		return nil, false
	}

	delta := chunk.Choices[0].Delta
	if len(delta.ToolCalls) > 0 {
		s.processToolCalls(delta.ToolCalls)
		return nil, false
	}

	return s.parseContent(delta)
}

func (s *streamSession) parseUsage(usage *openRouterChatChunkUsage) {
	if usage == nil {
		return
	}

	if usage.PromptTokens >= 0 && usage.PromptTokens <= math.MaxUint32 {
		s.inputTokens = uint32(usage.PromptTokens)
	}

	if usage.CompletionTokens >= 0 && usage.CompletionTokens <= math.MaxUint32 {
		s.outputTokens = uint32(usage.CompletionTokens)
	}

	s.totalPrice = decimal.NewFromFloat(usage.Cost)
}

func (s *streamSession) processToolCalls(toolCalls []openRouterToolCall) {
	for _, tc := range toolCalls {
		s.updateToolCallAccumulator(tc)
	}
}

func (s *streamSession) updateToolCallAccumulator(tc openRouterToolCall) {
	acc, ok := s.toolCalls[tc.Index]
	if !ok {
		acc = &toolCallAccumulator{}
		s.toolCalls[tc.Index] = acc
	}

	if tc.ID != nil && *tc.ID != "" {
		acc.id = *tc.ID
	}

	if tc.Function == nil {
		return
	}

	if tc.Function.Name != nil && *tc.Function.Name != "" {
		acc.name = *tc.Function.Name
	}

	if tc.Function.Arguments != nil && *tc.Function.Arguments != "" {
		acc.arguments.WriteString(*tc.Function.Arguments)
	}
}

func (s *streamSession) parseContent(
	delta openRouterChatChunkChoiceDelta,
) (messages.Message, bool) {
	var content, reasoning string
	if delta.Content != nil {
		content = *delta.Content
	}

	if delta.Reasoning != nil {
		reasoning = *delta.Reasoning
	}

	if content == "" && reasoning == "" {
		return nil, false
	}

	s.outputTokens++ // fallback estimate

	msg, err := messages.NewMessageAssistant(
		content,
		messages.WithMessageAssistantMergeTag(0),
		messages.WithMessageAssistantReasoning(reasoning),
		messages.WithMessageAssistantAgentID(s.agentID),
	)
	if err != nil {
		s.err = fmt.Errorf("failed to create assistant message: %w", err)
		return nil, false
	}

	return msg, true
}

func (s *streamSession) makeToolRequestMessage(acc *toolCallAccumulator) (messages.Message, error) {
	argsRaw := make(map[string]json.RawMessage)
	if acc.arguments.Len() > 0 {
		if err := json.Unmarshal([]byte(acc.arguments.String()), &argsRaw); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tool arguments: %w", err)
		}
	}

	callID := acc.id
	if callID == "" {
		callID = uuid.NewString()
	}

	msg, err := messages.NewMessageToolRequest(
		argsRaw, acc.name, callID,
		messages.WithMessageToolRequestMergeTag(0),
	)
	if err != nil {
		return nil, fmt.Errorf("creating message tool request: %w", err)
	}

	return msg, nil
}

func (s *streamSession) yieldToolCalls() (messages.Message, bool) {
	if !s.yieldedTools && len(s.toolCalls) > 0 {
		s.yieldedTools = true

		keys := make([]int64, 0, len(s.toolCalls))
		for k := range s.toolCalls {
			keys = append(keys, k)
		}

		slices.Sort(keys)

		for _, k := range keys {
			if msg, err := s.makeToolRequestMessage(s.toolCalls[k]); err == nil {
				s.pendingMessages = append(s.pendingMessages, msg)
			}
		}
	}

	if len(s.pendingMessages) > 0 {
		msg := s.pendingMessages[0]
		s.pendingMessages = s.pendingMessages[1:]

		return msg, true
	}

	s.finished = true

	return nil, false
}

func (s *streamSession) Close() (chatmodel.UsageStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx := &valueContext{valuer: s.ctx}
	errClose := s.body.Close()

	if s.inputTokens > 0 && s.inputTokens != s.wantInputTokens {
		s.adapter.obs.tokenCountMismatch(ctx, s.model, s.wantInputTokens, s.inputTokens)
	}

	stats := chatmodel.UsageStats{
		InputTokens:  s.inputTokens,
		OutputTokens: s.outputTokens,
		Duration:     time.Since(s.startTime),
		CostUSD:      s.totalPrice,
	}

	if errClose != nil {
		return stats, errors.Join(s.err, errClose)
	}

	return stats, s.err
}

type valueContext struct {
	valuer interface{ Value(key any) any }
}

var _ context.Context = (*valueContext)(nil)

func (*valueContext) Deadline() (deadline time.Time, ok bool) { return deadline, ok }
func (*valueContext) Done() <-chan struct{}                   { return nil }
func (*valueContext) Err() error                              { return nil }

//nolint:ireturn // interface requirement
func (v *valueContext) Value(key any) any { return v.valuer.Value(key) }

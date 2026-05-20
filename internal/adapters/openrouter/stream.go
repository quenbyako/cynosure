package openrouter

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

	totalTokens, err := o.checkPreflightAndRateLimits(
		ctx, settings, params.PreflightCheck(), params.Toolbox(), input,
	)
	if err != nil {
		return nil, err
	}

	req, err := o.buildChatRequest(settings, params.Toolbox(), params.ToolChoice(), input)
	if err != nil {
		return nil, err
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://openrouter.ai/api/v1/chat/completions", bytes.NewReader(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	httpReq.Header.Set("Accept", "application/json;q=1, text/event-stream;q=0")
	httpReq.Header.Set("User-Agent", "speakeasy-sdk/go 0.4.1 2.879.6 1.0.0 github.com/OpenRouterTeam/go-sdk")

	resp, err := o.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openrouter request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()

		bodyBytes, _ := io.ReadAll(resp.Body)

		return nil, fmt.Errorf("openrouter request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return o.newStreamSession(ctx, settings, resp.Body, uint32(totalTokens)), nil
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

	totalTokens, err := o.tokenCounter.CountTokens(settings.Model(), tokMsgs)
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
	for i, t := range toolList {
		var paramsMap map[string]any

		_ = json.Unmarshal(t.ConvertedSchema(), &paramsMap)

		desc := t.Desc()
		req.Tools[i] = components.CreateChatFunctionToolChatFunctionToolFunction(components.ChatFunctionToolFunction{
			Type: components.ChatFunctionToolTypeFunction,
			Function: components.ChatFunctionToolFunctionFunction{
				Name:        t.Name(),
				Description: &desc,
				Parameters:  paramsMap,
			},
		})
	}

	choice, err := datatransfer.ConvertToolChoice(toolChoice)
	if err != nil {
		return err
	}

	req.ToolChoice = &choice

	return nil
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
		model:           settings.Model(),
		agentID:         settings.ID(),
		body:            body,
		scanner:         bufio.NewScanner(body),
		toolCalls:       make(map[int64]*toolCallAccumulator),
		yieldedTools:    false,
		wantInputTokens: wantInputTokens,
	}
}

func (o *Adapter) waitInputLimit(ctx context.Context, numTokens int) error {
	cause := chatmodel.ErrHardQuotaExhausted

	limCtx, cancel := context.WithTimeoutCause(ctx, maxLimiterWait, cause)
	defer cancel()

	if err := o.chatInputLimiter.WaitN(limCtx, numTokens); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(context.Cause(limCtx), context.Canceled) {
			return err
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

type openRouterChatChunk struct {
	ID      string `json:"id"`
	Choices []struct {
		Delta struct {
			Content   *string `json:"content"`
			Reasoning *string `json:"reasoning"`
			ToolCalls []struct {
				Index    int64   `json:"index"`
				ID       *string `json:"id"`
				Type     *string `json:"type"`
				Function *struct {
					Name      *string `json:"name"`
					Arguments *string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int64   `json:"prompt_tokens"`
		CompletionTokens int64   `json:"completion_tokens"`
		TotalTokens      int64   `json:"total_tokens"`
		Cost             float64 `json:"cost"`
	} `json:"usage"`
}

type streamSession struct {
	ctx             interface{ Value(key any) any }
	startTime       time.Time
	adapter         *Adapter
	body            io.ReadCloser
	scanner         *bufio.Scanner
	err             error
	toolCalls       map[int64]*toolCallAccumulator
	pendingMessages []messages.Message
	model           string
	mu              sync.Mutex
	wantInputTokens uint32
	inputTokens     uint32
	outputTokens    uint32
	totalPrice      decimal.Decimal
	agentID         ids.AgentID
	yieldedTools    bool
	finished        bool
}

var _ chatmodel.Iter = (*streamSession)(nil)

func (s *streamSession) Next() (messages.Message, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.finished || s.err != nil {
		return nil, false
	}

	if len(s.pendingMessages) > 0 {
		msg := s.pendingMessages[0]
		s.pendingMessages = s.pendingMessages[1:]

		return msg, true
	}

	if s.yieldedTools {
		return nil, false
	}

	for s.scanner.Scan() {
		line := s.scanner.Text()
		if line == "" || strings.HasPrefix(line, ":") { // Skip keepalive comments
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk openRouterChatChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			s.err = err
			return nil, false
		}

		if msg, ok := s.processStreamChunk(chunk); ok {
			return msg, true
		}
	}

	if err := s.scanner.Err(); err != nil {
		s.err = err
		return nil, false
	}

	return s.yieldToolCalls()
}

func (s *streamSession) processStreamChunk(chunk openRouterChatChunk) (messages.Message, bool) {
	if chunk.Usage != nil {
		s.inputTokens = uint32(chunk.Usage.PromptTokens)
		s.outputTokens = uint32(chunk.Usage.CompletionTokens)
		s.totalPrice = decimal.NewFromFloat(chunk.Usage.Cost)
	}

	if len(chunk.Choices) == 0 {
		return nil, false
	}

	delta := chunk.Choices[0].Delta
	if len(delta.ToolCalls) > 0 {
		for _, tc := range delta.ToolCalls {
			idx := tc.Index

			acc, ok := s.toolCalls[idx]
			if !ok {
				acc = &toolCallAccumulator{}
				s.toolCalls[idx] = acc
			}

			if tc.ID != nil && *tc.ID != "" {
				acc.id = *tc.ID
			}

			if tc.Function != nil && tc.Function.Name != nil && *tc.Function.Name != "" {
				acc.name = *tc.Function.Name
			}

			if tc.Function != nil && tc.Function.Arguments != nil && *tc.Function.Arguments != "" {
				acc.arguments.WriteString(*tc.Function.Arguments)
			}
		}

		return nil, false
	}

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
		s.err = err
		return nil, false
	}

	return msg, true
}

func (s *streamSession) makeToolRequestMessage(acc *toolCallAccumulator) (messages.Message, error) {
	argsRaw := make(map[string]json.RawMessage)
	if acc.arguments.Len() > 0 {
		_ = json.Unmarshal([]byte(acc.arguments.String()), &argsRaw)
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
	_ = s.body.Close()

	if s.inputTokens > 0 && s.inputTokens != s.wantInputTokens {
		s.adapter.obs.tokenCountMismatch(ctx, s.model, s.wantInputTokens, s.inputTokens)
	}

	stats := chatmodel.UsageStats{
		InputTokens:  s.inputTokens,
		OutputTokens: s.outputTokens,
		Duration:     time.Since(s.startTime),
		CostUSD:      s.totalPrice,
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

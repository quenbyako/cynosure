# Research: Telegram-to-A2A Gateway Implementation

**Feature**: 001-telegram-a2a-gateway
**Phase**: 0 - Research & Technical Decisions
**Date**: 2025-11-07

## Overview

This document consolidates research findings for completing the Telegram-to-A2A gateway implementation. Focus areas include: Go concurrency best practices for streaming, Telegram Bot API rate limiting patterns, A2A protocol buffer parsing, and error handling strategies.

---

## 1. Go Concurrency Patterns for Streaming Responses

### Problem
Current implementation has a critical bug: uses non-existent `wg.Go()` method and never closes the channel feeding `SendMessage`, causing goroutine leaks and incorrect synchronization.

### Research Findings

**Pattern 1: Channel-Based Producer-Consumer with WaitGroup**

```go
func processStream(ctx context.Context, stream iter.Seq2[T, error]) chan T {
    out := make(chan T)
    var wg sync.WaitGroup

    wg.Add(1)
    go func() {
        defer wg.Done()
        defer close(out)  // CRITICAL: close channel when done

        for item, err := range stream {
            if err != nil {
                // Handle error
                return
            }
            select {
            case out <- item:
            case <-ctx.Done():
                return
            }
        }
    }()

    // Don't wait here - consumer will drain channel
    return out
}
```

**Pattern 2: Context Cancellation for Cleanup**

For proper cleanup when the application shuts down:

```go
func (u *Usecase) ReceiveNewMessageEvent(ctx context.Context, msg *entities.Message) error {
    ctx, cancel := context.WithCancel(ctx)
    defer cancel() // Ensures cleanup on return

    respStream, err := u.a2a.SendMessage(ctx, msg.ID(), text)
    if err != nil {
        return err
    }

    textChan := make(chan components.MessageText)

    go func() {
        defer close(textChan)
        for part, err := range respStream {
            select {
            case <-ctx.Done():
                return
            default:
                if err != nil {
                    // Send error to channel or log
                    continue
                }
                textChan <- part
            }
        }
    }()

    return u.client.SendMessage(ctx, msg.ID().ChannelID(), textChan)
}
```

### Decision: Use Pattern 2 with Context Cancellation

**Rationale**:
- Explicit cleanup via `defer cancel()`
- Proper channel closure prevents goroutine leaks
- Context propagation enables graceful shutdown
- Follows standard Go patterns from official documentation

**Implementation**:
- Fix `internal/domains/gateway/usecases/usecase.go:35-75`
- Replace `wg.Go()` with proper goroutine + context handling
- Add `defer close(textChan)` after goroutine completes

**Alternatives Considered**:
- **errgroup.Group**: Overkill for single goroutine, adds dependency
- **sync.WaitGroup without context**: No graceful shutdown support

**References**:
- Go Concurrency Patterns (blog.golang.org)
- Effective Go: Channels section
- Go by Example: Worker Pools

---

## 2. Telegram Bot API Message Streaming Patterns

### Problem
Current implementation accumulates entire response before sending. Need to update messages progressively (~3 second intervals) for better UX.

### Research Findings

**Telegram API Constraints**:
- Maximum 20 messages per second per bot
- Message editing: ~1 edit per message per second (soft limit)
- Message length limit: 4096 characters (UTF-8)
- `editMessageText` method requires: `chat_id`, `message_id`, `text`

**Pattern: Time-Based Batching with Last Update Tracking**

```go
type StreamingMessenger struct {
    api *tgbotapi.BotAPI
    updateInterval time.Duration
}

func (m *StreamingMessenger) SendMessage(ctx context.Context, channelID ids.ChannelID, textChan chan components.MessageText) error {
    chatID, _ := parseChatID(channelID)

    var accumulated strings.Builder
    var messageID int
    ticker := time.NewTicker(3 * time.Second)
    defer ticker.Stop()

    // Send initial message immediately
    firstText := <-textChan
    accumulated.WriteString(firstText.Text())

    msg, err := m.api.Send(tgbotapi.NewMessage(chatID, accumulated.String()))
    if err != nil {
        return err
    }
    messageID = msg.MessageID

    // Stream updates
    for {
        select {
        case text, ok := <-textChan:
            if !ok {
                // Final update
                return m.editMessage(chatID, messageID, accumulated.String())
            }
            accumulated.WriteString(text.Text())

        case <-ticker.C:
            // Periodic update
            if accumulated.Len() > 0 {
                m.editMessage(chatID, messageID, accumulated.String())
            }

        case <-ctx.Done():
            return ctx.Err()
        }
    }
}

func (m *StreamingMessenger) editMessage(chatID int64, messageID int, text string) error {
    edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
    _, err := m.api.Send(edit)
    if err != nil && strings.Contains(err.Error(), "message is not modified") {
        return nil // Ignore "not modified" errors
    }
    return err
}
```

**Rate Limiting Strategy**:
- Use `time.Ticker` for fixed 3-second intervals
- Track last edit time to avoid hitting rate limits
- Gracefully handle "message is not modified" errors (Telegram returns this if content identical)

### Decision: Implement Time-Based Batching with Ticker

**Rationale**:
- Simple, predictable user experience (updates every 3 seconds)
- Stays well within Telegram rate limits (max 20 edits/minute vs Telegram's ~60/minute limit)
- Handles partial updates naturally
- Easy to test and reason about

**Implementation**:
- Rewrite `internal/adapters/telegram/client.go:SendMessage` method
- Add initial message send + streaming edit loop
- Handle channel closure for final update

**Alternatives Considered**:
- **Token bucket rate limiter**: Over-engineered for current scale
- **Adaptive timing** (speed up if no new data): Adds complexity without user benefit
- **Multiple messages instead of editing**: Clutters chat, poor UX

**Edge Cases to Handle**:
1. Very fast streams: Ticker ensures we don't edit too frequently
2. Very slow streams: User sees updates as they arrive (every 3s)
3. Message too long (>4096 chars): Truncate with "..." indicator
4. Network errors during edit: Log and continue (don't fail entire stream)

**References**:
- Telegram Bot API Documentation: editMessageText
- go-telegram-bot-api library documentation
- Stack Overflow: Telegram message update patterns

---

## 3. A2A Protocol Buffer Response Parsing

### Problem
Current code uses `resp.GetMsg().String()` which returns debug representation, not actual text content.

### Research Findings

**A2A Message Structure** (from protobuf definition):

```protobuf
message Message {
  string message_id = 1;
  string context_id = 2;
  Role role = 3;
  repeated Part content = 4;
}

message Part {
  oneof part {
    string text = 1;
    ToolCall tool_call = 2;
    ToolResponse tool_response = 3;
  }
}
```

**Correct Parsing Pattern**:

```go
func extractTextFromMessage(msg *a2a.Message) (string, error) {
    var text strings.Builder

    for _, part := range msg.GetContent() {
        switch p := part.GetPart().(type) {
        case *a2a.Part_Text:
            text.WriteString(p.Text)
        case *a2a.Part_ToolCall:
            // Optionally format tool call for user display
            // For now, skip (out of scope for first iteration)
        case *a2a.Part_ToolResponse:
            // Skip tool responses in user-facing output
        default:
            // Unknown part type - log warning but continue
        }
    }

    if text.Len() == 0 {
        return "", fmt.Errorf("message contains no text content")
    }

    return text.String(), nil
}
```

**Streaming Response Handling**:

```go
func (c *Client) SendMessage(ctx context.Context, chat ids.MessageID, text components.MessageText) (iter.Seq2[components.MessageText, error], error) {
    stream, err := c.client.SendStreamingMessage(ctx, &a2a.SendMessageRequest{
        Request: &a2a.Message{
            MessageId: chat.String(),
            ContextId: chat.ChannelID().String(),
            Role:      a2a.Role_ROLE_USER,
            Content:   []*a2a.Part{{Part: &a2a.Part_Text{Text: text.Text()}}},
        },
    })
    if err != nil {
        return nil, err
    }

    return func(yield func(components.MessageText, error) bool) {
        for {
            resp, err := stream.Recv()
            if errors.Is(err, io.EOF) {
                return
            }
            if err != nil {
                yield(components.MessageText{}, err)
                return
            }

            // Extract text from response message
            textContent, err := extractTextFromMessage(resp.GetMsg())
            if err != nil {
                yield(components.MessageText{}, err)
                return
            }

            msgText, err := components.NewMessageText(textContent)
            if err != nil {
                yield(components.MessageText{}, err)
                return
            }

            if !yield(msgText, nil) {
                return
            }
        }
    }, nil
}
```

### Decision: Parse Part.Text Fields from Content Array

**Rationale**:
- Follows A2A protocol specification correctly
- Handles multiple parts in single message (concatenate text parts)
- Extensible to tool calls in future iterations
- Returns structured errors for invalid content

**Implementation**:
- Fix `internal/adapters/a2a/client.go:SendMessage` method
- Add `extractTextFromMessage` helper function
- Replace `.String()` with proper Part parsing

**Alternatives Considered**:
- **Marshal to JSON**: Inefficient, loses type safety
- **Accept first part only**: Breaks if A2A sends multi-part messages
- **Display tool calls as JSON**: Out of scope, clutters user experience

**Edge Cases**:
1. Message with only tool calls, no text: Return error to user
2. Multiple text parts: Concatenate with no separator
3. Unknown part types: Log warning, skip gracefully
4. Empty content array: Return error "no content received"

**References**:
- A2A Protocol Specification (google.golang.org/a2a)
- Protocol Buffers Language Guide (protobuf.dev)
- Existing A2A controller implementation in codebase

---

## 4. Error Handling and User Notifications

### Problem
Errors currently suppressed with TODO comments. Users see no feedback when things fail.

### Research Findings

**Error Categories**:
1. **Infrastructure errors**: A2A server unavailable, network timeouts
2. **Validation errors**: Empty messages, invalid IDs
3. **Rate limiting**: Telegram API throttling
4. **Parsing errors**: Invalid A2A responses

**User-Friendly Error Messages**:

```go
type ErrorCategory int

const (
    ErrCategoryInfrastructure ErrorCategory = iota
    ErrCategoryValidation
    ErrCategoryRateLimit
    ErrCategoryParsing
)

func userFriendlyError(err error, category ErrorCategory) string {
    switch category {
    case ErrCategoryInfrastructure:
        if errors.Is(err, context.DeadlineExceeded) {
            return "⏱ The agent is taking too long to respond. Please try again."
        }
        if status.Code(err) == codes.Unavailable {
            return "🔌 The agent service is temporarily unavailable. Please try again in a few moments."
        }
        return "❌ An unexpected error occurred. Please try again."

    case ErrCategoryValidation:
        return "⚠️ Your message appears to be invalid. Please check and try again."

    case ErrCategoryRateLimit:
        return "⏸ Too many requests. Please wait a moment before sending another message."

    case ErrCategoryParsing:
        return "⚙️ Received an unexpected response format. Our team has been notified."

    default:
        return "❌ Something went wrong. Please try again."
    }
}
```

**Structured Logging Pattern**:

```go
type LogCallbacks interface {
    ProcessMessageStart(ctx context.Context, chatID int64, messageID string)
    ProcessMessageSuccess(ctx context.Context, chatID int64, messageID string, duration time.Duration)
    ProcessMessageIssue(ctx context.Context, chatID int64, err error)
}

type structuredLogger struct {
    logger *slog.Logger
}

func (l *structuredLogger) ProcessMessageIssue(ctx context.Context, chatID int64, err error) {
    l.logger.ErrorContext(ctx, "message processing failed",
        slog.String("domain", "gateway"),
        slog.Int64("chat_id", chatID),
        slog.String("error", err.Error()),
        slog.String("error_type", categorizeError(err)),
    )
}
```

### Decision: Categorize Errors + User-Friendly Messages + Structured Logging

**Rationale**:
- Users get clear, actionable feedback
- Developers get detailed context for debugging
- Error categories enable metrics/alerting
- Emojis improve user experience in chat interface

**Implementation**:
1. Add error categorization in `internal/domains/gateway/usecases/usecase.go`
2. Create `userFriendlyError` helper function
3. Replace `pp.Println` with structured logger in controllers
4. Send friendly errors to users via Telegram

**Alternatives Considered**:
- **Generic errors only**: Poor user experience, hard to debug
- **Detailed technical errors to users**: Security risk, confusing
- **Silent failures with logs**: Terrible UX, users think bot is broken

**Error Propagation Strategy**:
```
A2A Adapter Error → Usecase (categorize + log) → Controller (user message) → Telegram
```

**Edge Cases**:
1. Error during error notification: Log and swallow (prevent infinite loops)
2. Partial success (some streaming chunks delivered): Send error after last chunk
3. Concurrent errors: Log all, send most recent to user

**References**:
- Go Error Handling Best Practices (go.dev/blog)
- User Experience Patterns for Error Messages
- Structured Logging with slog (Go 1.21+)

---

## 5. Testing Strategy

### Problem
Existing code has minimal test coverage. Need comprehensive tests before merge.

### Research Findings

**Test Pyramid for Gateway**:
1. **Unit Tests** (~60%): Test individual functions and components
2. **Contract Tests** (~30%): Test port implementations match interfaces
3. **Integration Tests** (~10%): Test full webhook→A2A→Telegram flow

**Contract Test Pattern for Ports**:

```go
// tests/contract/messenger_contract_test.go
func TestMessengerPort_BasicContract(t *testing.T) {
    tests := []struct {
        name        string
        messenger   ports.Messenger
        channelID   ids.ChannelID
        textChan    chan components.MessageText
        wantErr     bool
    }{
        {
            name:      "telegram adapter - simple message",
            messenger: setupTelegramAdapter(t),
            channelID: must(ids.NewChannelID("telegram", "123456")),
            textChan:  makeTextChan("Hello world"),
            wantErr:   false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            ctx := context.Background()
            err := tt.messenger.SendMessage(ctx, tt.channelID, tt.textChan)
            if (err != nil) != tt.wantErr {
                t.Errorf("SendMessage() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

**Streaming Test Pattern**:

```go
func TestTelegramAdapter_Streaming(t *testing.T) {
    mock := setupMockTelegramAPI(t)
    adapter := telegram.NewMessenger(mock)

    textChan := make(chan components.MessageText, 3)
    textChan <- must(components.NewMessageText("Part 1"))
    textChan <- must(components.NewMessageText(" Part 2"))
    textChan <- must(components.NewMessageText(" Part 3"))
    close(textChan)

    err := adapter.SendMessage(context.Background(), testChannelID, textChan)
    require.NoError(t, err)

    // Verify initial message sent
    assert.Equal(t, 1, mock.SendCallCount())

    // Verify at least one edit occurred
    assert.GreaterOrEqual(t, mock.EditMessageCallCount(), 1)

    // Verify final message contains all parts
    assert.Contains(t, mock.LastEditedMessage(), "Part 1 Part 2 Part 3")
}
```

**Integration Test with TestContainers** (optional):

```go
func TestGateway_E2E(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    // Start mock A2A server
    a2aServer := startMockA2AServer(t)
    defer a2aServer.Stop()

    // Start gateway
    gateway := startGatewayApp(t, a2aServer.Addr())
    defer gateway.Stop()

    // Simulate webhook
    webhook := sendWebhookUpdate(t, gateway.WebhookURL(), testMessage)

    // Verify A2A received message
    assert.Eventually(t, func() bool {
        return a2aServer.ReceivedMessageCount() > 0
    }, 5*time.Second, 100*time.Millisecond)

    // Verify response sent to Telegram
    // (requires Telegram API mock)
}
```

### Decision: Implement 3-Level Test Strategy

**Test Coverage Requirements**:
- **Unit tests**: All concurrency fixes, error handling, parsing logic
- **Contract tests**: Both ports (Agent, Messenger) with real implementations
- **Integration test**: Simplified webhook→usecase→adapters flow (no network)

**Implementation Priority**:
1. Unit tests for critical bugs (concurrency, parsing)
2. Contract tests for ports
3. Integration test (stretch goal, optional for MVP)

**Mocking Strategy**:
- Use interface-based mocks (no reflection frameworks)
- Mock Telegram API for contract tests
- Mock A2A gRPC client for contract tests

**Alternatives Considered**:
- **End-to-end tests only**: Brittle, slow, hard to debug
- **Table-driven tests everywhere**: Overkill for simple cases
- **TestContainers for all tests**: Too slow for CI, optional

**References**:
- Go Testing Best Practices (golang.org)
- Contract Testing Patterns
- Table-Driven Tests in Go

---

## 6. Observability and Metrics

### Problem
Current implementation uses `pp.Println` for debugging. Need structured logging and metrics.

### Research Findings

**Logging Framework**: Use Go 1.21+ `log/slog` (already standard library)

**Key Metrics to Track**:
1. **Message throughput**: Messages/second per chat
2. **Streaming duration**: Time from first chunk to last
3. **Error rates**: By error category
4. **A2A latency**: Time to first response chunk
5. **Telegram API rate limit hits**: Track 429 responses

**Implementation Pattern**:

```go
// internal/controllers/tgbot/logs.go
type LogCallbacks struct {
    logger *slog.Logger
    metrics MetricsCollector
}

func (l *LogCallbacks) ProcessMessageStart(ctx context.Context, chatID int64, messageID string) {
    l.logger.InfoContext(ctx, "processing message started",
        slog.String("domain", "gateway"),
        slog.Int64("chat_id", chatID),
        slog.String("message_id", messageID),
    )
    l.metrics.IncCounter("gateway.messages.started", map[string]string{
        "chat_id": strconv.FormatInt(chatID, 10),
    })
}

func (l *LogCallbacks) ProcessMessageSuccess(ctx context.Context, chatID int64, messageID string, duration time.Duration) {
    l.logger.InfoContext(ctx, "processing message completed",
        slog.String("domain", "gateway"),
        slog.Int64("chat_id", chatID),
        slog.String("message_id", messageID),
        slog.Duration("duration", duration),
    )
    l.metrics.RecordDuration("gateway.messages.duration", duration, map[string]string{
        "chat_id": strconv.FormatInt(chatID, 10),
    })
}
```

### Decision: Add Structured Logging with slog

**Rationale**:
- slog is standard library (no new dependencies)
- Structured logs enable log aggregation/search
- Context propagation for distributed tracing
- Minimal performance overhead

**Implementation**:
1. Replace `pp.Println` with slog calls
2. Add context keys: domain, chat_id, message_id, action
3. Use appropriate log levels (Info, Error, Warn)
4. Optional: Add metrics via existing observability framework

**Metrics** (optional, use existing framework if available):
- Defer to Phase 2 if observability framework needs setup
- If `core.Metrics` interface exists, use it

**References**:
- slog Package Documentation (pkg.go.dev)
- Structured Logging Best Practices
- OpenTelemetry for Go (future enhancement)

---

## Summary of Decisions

| Area | Decision | Rationale |
|------|----------|-----------|
| **Concurrency** | Context-based cancellation with proper channel closure | Standard Go pattern, enables cleanup |
| **Streaming** | Time-based batching with 3s ticker | Simple, predictable, within rate limits |
| **Parsing** | Extract text from A2A Part messages | Follows protocol specification |
| **Error Handling** | Categorize + user-friendly messages + structured logs | Best UX + debuggability |
| **Testing** | Unit + contract + integration tests | Comprehensive coverage, test pyramid |
| **Observability** | Structured logging with slog | Standard library, minimal overhead |

---

## Next Steps

**Phase 1**: Generate data model, contracts, and quickstart documentation based on these decisions.

**Implementation Order**:
1. Fix concurrency bugs (highest priority - prevents goroutine leaks)
2. Fix A2A parsing (enables correct responses)
3. Implement streaming (improves UX)
4. Add error handling (improves reliability)
5. Add tests (ensures quality)
6. Add observability (enables monitoring)

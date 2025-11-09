# Messenger Port Contract

**Feature**: 001-telegram-a2a-gateway
**Port**: `internal/domains/gateway/ports/messenger.go`
**Purpose**: Abstracts messaging platform (Telegram) communication

---

## Interface Definition

```go
package ports

import (
    "context"
    "github.com/quenbyako/cynosure/internal/domains/gateway/components"
    "github.com/quenbyako/cynosure/internal/domains/gateway/components/ids"
)

type Messenger interface {
    // SendMessage sends a message with streaming updates to a channel.
    //
    // Parameters:
    //   - ctx: Context for cancellation and timeout
    //   - channelID: Target channel (Telegram chat)
    //   - text: Channel providing message text chunks
    //
    // Returns:
    //   - Error if message cannot be sent
    //
    // Behavior:
    //   - Sends initial message with first chunk immediately
    //   - Updates message progressively as chunks arrive (~3 second intervals)
    //   - Sends final update when text channel closes
    //   - Respects platform rate limits internally
    //
    SendMessage(ctx context.Context, channelID ids.ChannelID, text chan components.MessageText) error

    // NotifyProcessingStarted sends a "typing" indicator to the channel.
    //
    // Parameters:
    //   - ctx: Context for cancellation
    //   - channelID: Target channel (Telegram chat)
    //
    // Returns:
    //   - Error if notification cannot be sent
    //
    NotifyProcessingStarted(ctx context.Context, channelID ids.ChannelID) error
}
```

---

## Contract Requirements

### 1. SendMessage Semantics

**Input Validation**:
- `channelID` MUST be valid (non-empty provider and channel)
- `text` channel MUST NOT be nil
- Implementation MUST return error immediately if channelID invalid
- Implementation SHOULD validate channelID provider (e.g., "telegram" only)

**Channel Consumption**:
- Implementation MUST consume all values from `text` channel until closed
- Implementation MUST NOT close the `text` channel (caller responsibility)
- Implementation SHOULD handle channel closure gracefully

**Message Flow**:
1. **Initial Send**: Send first chunk as new message immediately
2. **Streaming Updates**: Edit message with accumulated text every ~3 seconds
3. **Final Update**: Edit message with complete accumulated text when channel closes
4. **Rate Limiting**: Respect platform limits (Telegram: ~1 edit/second per message)

**Accumulation Strategy**:
```go
// Caller sends incremental chunks
textChan <- MessageText("Hello")
textChan <- MessageText(" world")
textChan <- MessageText("!")
close(textChan)

// Implementation accumulates:
// Initial message: "Hello"
// Update (after 3s): "Hello world"
// Final update: "Hello world!"
```

### 2. NotifyProcessingStarted Semantics

**Purpose**: Provide immediate user feedback that message is being processed.

**Implementation**:
- Send "typing" indicator (Telegram: `sendChatAction` with `typing`)
- Indicator typically visible for ~5 seconds on Telegram
- Errors SHOULD be logged but not fail the entire request

**Timing**:
- MUST be called before forwarding message to A2A
- SHOULD complete within 500ms
- MAY be called multiple times (implementation should handle idempotency)

### 3. Rate Limiting & Throttling

**Platform Limits** (Telegram):
- Maximum 20 messages per second per bot
- Message editing: ~1 edit per message per second (soft limit)
- Repeated identical edits may be rejected ("message is not modified")

**Implementation Requirements**:
- MUST throttle updates to comply with platform limits
- SHOULD use time-based batching (e.g., `time.Ticker` for 3-second intervals)
- SHOULD track last edit time to avoid rate limit violations
- MUST handle "message is not modified" errors gracefully (not fail)

**Backpressure Handling**:
- If chunks arrive faster than rate limit allows, MUST batch multiple chunks
- If chunks arrive slower than update interval, SHOULD send updates as available

### 4. Error Handling

**Error Categories**:

| Error Type | When | Action |
|------------|------|--------|
| Invalid channelID | Before sending | Return error immediately |
| Platform unavailable | During send | Return error (retryable) |
| Rate limit exceeded | During updates | Retry with backoff |
| Message too long | During accumulation | Truncate with indicator |
| Network timeout | Anytime | Return error |

**Error Recovery**:
- Implementation MUST NOT panic on errors
- Implementation SHOULD retry transient errors (rate limits, network) with exponential backoff
- Implementation SHOULD log errors for observability
- Implementation MAY send partial content on critical errors (rather than fail completely)

**Message Length Handling**:
- Telegram limit: 4096 characters (UTF-8)
- If accumulated text exceeds limit, MUST truncate with indicator (e.g., "...[truncated]")
- Alternative: Split into multiple messages (optional enhancement)

### 5. Concurrency & Resource Management

**Goroutine Safety**:
- Implementation MUST be safe for concurrent calls (different channels)
- Implementation MAY serialize calls to same channelID (optional optimization)

**Resource Cleanup**:
- Implementation MUST close HTTP/gRPC connections properly
- Implementation MUST respect context cancellation
- Implementation MUST drain `text` channel even if send fails (avoid caller goroutine leak)

**Context Handling**:
- Implementation MUST stop processing on context cancellation
- Implementation SHOULD use `select` with `<-ctx.Done()` in update loop

---

## Contract Tests

### Test 1: Basic Message Send

```go
func TestMessengerContract_BasicSend(t *testing.T) {
    messenger := setupMessengerImplementation(t)
    ctx := context.Background()

    channelID := must(ids.NewChannelID("telegram", "123456"))
    textChan := make(chan components.MessageText, 1)
    textChan <- must(components.NewMessageText("Hello"))
    close(textChan)

    err := messenger.SendMessage(ctx, channelID, textChan)
    assert.NoError(t, err)
}
```

### Test 2: Streaming Updates

```go
func TestMessengerContract_Streaming(t *testing.T) {
    mockAPI := setupMockTelegramAPI(t)
    messenger := setupMessengerWithMock(t, mockAPI)
    ctx := context.Background()

    textChan := make(chan components.MessageText, 3)
    textChan <- must(components.NewMessageText("Part 1"))
    time.Sleep(3 * time.Second)
    textChan <- must(components.NewMessageText(" Part 2"))
    time.Sleep(3 * time.Second)
    textChan <- must(components.NewMessageText(" Part 3"))
    close(textChan)

    err := messenger.SendMessage(ctx, testChannelID, textChan)
    require.NoError(t, err)

    // Verify initial send occurred
    assert.Equal(t, 1, mockAPI.NewMessageCallCount())

    // Verify at least one edit occurred
    assert.GreaterOrEqual(t, mockAPI.EditMessageCallCount(), 1)

    // Verify final message contains all parts
    finalMessage := mockAPI.LastMessageText()
    assert.Contains(t, finalMessage, "Part 1")
    assert.Contains(t, finalMessage, "Part 2")
    assert.Contains(t, finalMessage, "Part 3")
}
```

### Test 3: Rate Limiting Compliance

```go
func TestMessengerContract_RateLimiting(t *testing.T) {
    mockAPI := setupMockTelegramAPI(t)
    messenger := setupMessengerWithMock(t, mockAPI)

    // Send many chunks rapidly
    textChan := make(chan components.MessageText, 100)
    for i := 0; i < 100; i++ {
        textChan <- must(components.NewMessageText(fmt.Sprintf("Chunk %d ", i)))
    }
    close(textChan)

    start := time.Now()
    err := messenger.SendMessage(context.Background(), testChannelID, textChan)
    duration := time.Since(start)

    require.NoError(t, err)

    // Verify edits were throttled (not 100 edits in <1 second)
    editCount := mockAPI.EditMessageCallCount()
    assert.Less(t, editCount, 50, "should batch updates, not edit 100 times")

    // Verify took reasonable time (batching with 3s intervals)
    assert.Greater(t, duration, 2*time.Second, "should take time due to batching")
}
```

### Test 4: Message Length Truncation

```go
func TestMessengerContract_LongMessage(t *testing.T) {
    messenger := setupMessengerImplementation(t)
    ctx := context.Background()

    // Send text exceeding Telegram limit (4096 chars)
    longText := strings.Repeat("A", 5000)
    textChan := make(chan components.MessageText, 1)
    textChan <- must(components.NewMessageText(longText))
    close(textChan)

    err := messenger.SendMessage(ctx, testChannelID, textChan)
    assert.NoError(t, err) // Should truncate, not fail
}
```

### Test 5: Notify Processing Started

```go
func TestMessengerContract_NotifyProcessing(t *testing.T) {
    mockAPI := setupMockTelegramAPI(t)
    messenger := setupMessengerWithMock(t, mockAPI)
    ctx := context.Background()

    err := messenger.NotifyProcessingStarted(ctx, testChannelID)
    assert.NoError(t, err)

    // Verify typing indicator was sent
    assert.Equal(t, 1, mockAPI.ChatActionCallCount())
    assert.Equal(t, "typing", mockAPI.LastChatAction())
}
```

### Test 6: Context Cancellation

```go
func TestMessengerContract_ContextCancellation(t *testing.T) {
    messenger := setupMessengerImplementation(t)
    ctx, cancel := context.WithCancel(context.Background())

    textChan := make(chan components.MessageText)

    go func() {
        time.Sleep(1 * time.Second)
        cancel() // Cancel mid-stream
        textChan <- must(components.NewMessageText("Test"))
        close(textChan)
    }()

    err := messenger.SendMessage(ctx, testChannelID, textChan)
    assert.Error(t, err) // Should return context.Canceled
    assert.ErrorIs(t, err, context.Canceled)
}
```

### Test 7: Invalid Input

```go
func TestMessengerContract_InvalidInput(t *testing.T) {
    messenger := setupMessengerImplementation(t)
    ctx := context.Background()

    tests := []struct {
        name      string
        channelID ids.ChannelID
    }{
        {"empty channelID", ids.ChannelID{}},
        {"wrong provider", must(ids.NewChannelID("discord", "123"))},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            textChan := make(chan components.MessageText, 1)
            textChan <- must(components.NewMessageText("Test"))
            close(textChan)

            err := messenger.SendMessage(ctx, tt.channelID, textChan)
            assert.Error(t, err)
        })
    }
}
```

---

## Implementation Checklist

When implementing the `Messenger` port:

- [ ] Validate channelID before sending (provider, non-empty ID)
- [ ] Parse channelID to extract platform-specific ID (e.g., Telegram chat ID)
- [ ] Consume `text` channel until closed
- [ ] Accumulate text from chunks (incremental, not replacing)
- [ ] Send initial message with first chunk
- [ ] Use `time.Ticker` for 3-second update intervals
- [ ] Edit existing message (not send new messages)
- [ ] Handle "message is not modified" errors gracefully
- [ ] Truncate messages exceeding 4096 characters
- [ ] Drain channel on errors to avoid goroutine leaks
- [ ] Respect context cancellation
- [ ] Implement `NotifyProcessingStarted` with "typing" indicator
- [ ] Add structured logging for observability
- [ ] Write contract tests to verify compliance

---

## Example Implementation Snippet

### Streaming with Time-Based Batching

```go
func (m *Messenger) SendMessage(ctx context.Context, channelID ids.ChannelID, text chan components.MessageText) error {
    chatID, err := parseChatID(channelID)
    if err != nil {
        return err
    }

    var accumulated strings.Builder
    ticker := time.NewTicker(3 * time.Second)
    defer ticker.Stop()

    // Send initial message
    firstChunk, ok := <-text
    if !ok {
        return errors.New("text channel closed without data")
    }
    accumulated.WriteString(firstChunk.Text())

    msg, err := m.api.Send(tgbotapi.NewMessage(chatID, accumulated.String()))
    if err != nil {
        return err
    }
    messageID := msg.MessageID

    // Stream updates
    for {
        select {
        case chunk, ok := <-text:
            if !ok {
                // Final update
                return m.editMessage(chatID, messageID, accumulated.String())
            }
            accumulated.WriteString(chunk.Text())

            // Truncate if too long
            if accumulated.Len() > 4096 {
                truncated := accumulated.String()[:4090] + "...[truncated]"
                accumulated.Reset()
                accumulated.WriteString(truncated)
            }

        case <-ticker.C:
            // Periodic update
            if err := m.editMessage(chatID, messageID, accumulated.String()); err != nil {
                // Log but continue
                log.Warn("failed to update message", "error", err)
            }

        case <-ctx.Done():
            return ctx.Err()
        }
    }
}

func (m *Messenger) editMessage(chatID int64, messageID int, text string) error {
    edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
    _, err := m.api.Send(edit)

    // Ignore "not modified" errors
    if err != nil && strings.Contains(err.Error(), "message is not modified") {
        return nil
    }
    return err
}
```

---

## References

- Telegram Bot API: `core.telegram.org/bots/api`
- go-telegram-bot-api: `github.com/go-telegram-bot-api/telegram-bot-api`
- Rate Limiting Patterns: Token bucket, leaky bucket algorithms

# Agent Port Contract

**Feature**: 001-telegram-a2a-gateway
**Port**: `internal/domains/gateway/ports/agent.go`
**Purpose**: Abstracts communication with A2A (Agent-to-Agent) servers

---

## Interface Definition

```go
package ports

import (
    "context"
    "iter"
    "github.com/quenbyako/cynosure/internal/domains/gateway/components"
    "github.com/quenbyako/cynosure/internal/domains/gateway/components/ids"
)

type Agent interface {
    // SendMessage forwards a user message to the A2A server and returns
    // a streaming iterator of response chunks.
    //
    // Parameters:
    //   - ctx: Context for cancellation and timeout
    //   - chat: Message ID containing channel context for conversation continuity
    //   - text: User's message text
    //
    // Returns:
    //   - Iterator yielding (MessageText, nil) for successful chunks
    //   - Iterator yielding (empty, error) on failure
    //   - Error if request cannot be initiated
    //
    // Behavior:
    //   - Iterator completes when stream ends (io.EOF)
    //   - Context cancellation stops streaming
    //   - Each chunk is incremental (caller must accumulate)
    //
    SendMessage(ctx context.Context, chat ids.MessageID, text components.MessageText) (iter.Seq2[components.MessageText, error], error)
}
```

---

## Contract Requirements

### 1. Request Semantics

**Input Validation**:
- `chat` MUST be valid (non-empty channel and message components)
- `text` MUST be valid (non-empty, within character limits)
- Implementation MUST return error immediately if inputs invalid

**Context Handling**:
- Implementation MUST respect context cancellation
- Implementation SHOULD implement timeout via context deadline
- Implementation MUST clean up resources on context cancellation

**A2A Protocol Mapping**:
```go
// message_id: chat.String() (e.g., "telegram:123456:42")
// context_id: chat.ChannelID().String() (e.g., "telegram:123456")
// role: ROLE_USER
// content: [{text: text.Text()}]
```

### 2. Streaming Behavior

**Iterator Protocol**:
```go
for chunk, err := range responseIterator {
    if err != nil {
        // Handle error, iterator will complete after this
        break
    }
    // Process chunk (accumulate text)
}
```

**Guarantees**:
- Iterator yields chunks in order received from A2A server
- Iterator MUST yield at least one chunk or one error before completing
- Iterator MUST complete (stop yielding) after first error
- Iterator MUST complete when A2A stream ends (io.EOF)
- Iterator MUST NOT block indefinitely (respect context timeout)

**Chunk Semantics**:
- Each chunk is **incremental** (not cumulative)
- Caller is responsible for accumulation
- Empty chunks (zero-length text) MAY be skipped by implementation

### 3. Error Handling

**Error Categories**:

| Error Type | When | Yielded via Iterator? |
|------------|------|----------------------|
| Invalid input | Immediately on call | No (returned as error) |
| A2A unavailable | Before stream starts | No (returned as error) |
| Connection lost | During streaming | Yes (yielded in iterator) |
| Parse error | During streaming | Yes (yielded in iterator) |
| Context cancelled | Anytime | Yes (yielded if started, else returned) |

**Error Wrapping**:
- Implementation SHOULD wrap errors with context: `fmt.Errorf("context: %w", err)`
- Implementation SHOULD preserve gRPC status codes for debugging

**Recovery**:
- Implementation MUST NOT panic on errors
- Implementation MUST close all resources (streams, connections) on error
- Implementation SHOULD log errors for observability

### 4. Performance Expectations

**Latency**:
- First chunk SHOULD arrive within A2A server latency + 100ms overhead
- Subsequent chunks SHOULD stream with minimal buffering (<50ms)

**Throughput**:
- Implementation SHOULD handle multiple concurrent messages (at least 10)
- Implementation SHOULD NOT block other messages while streaming

**Resource Management**:
- Implementation MUST close gRPC stream when iterator completes
- Implementation MUST NOT leak goroutines after context cancellation
- Implementation SHOULD reuse gRPC connections across messages

---

## Implementation Contract Tests

### Test 1: Basic Message Exchange

```go
func TestAgentContract_BasicExchange(t *testing.T) {
    agent := setupAgentImplementation(t)
    ctx := context.Background()

    chat := must(ids.NewMessageID(
        must(ids.NewChannelID("telegram", "123")),
        "42",
    ))
    text := must(components.NewMessageText("Hello"))

    iter, err := agent.SendMessage(ctx, chat, text)
    require.NoError(t, err)

    var chunks []string
    for chunk, err := range iter {
        require.NoError(t, err)
        chunks = append(chunks, chunk.Text())
    }

    assert.NotEmpty(t, chunks, "should receive at least one chunk")
}
```

### Test 2: Streaming Response

```go
func TestAgentContract_Streaming(t *testing.T) {
    agent := setupAgentImplementation(t)
    ctx := context.Background()

    iter, err := agent.SendMessage(ctx, testChat, testText)
    require.NoError(t, err)

    chunkCount := 0
    for _, err := range iter {
        require.NoError(t, err)
        chunkCount++
    }

    assert.GreaterOrEqual(t, chunkCount, 1)
}
```

### Test 3: Context Cancellation

```go
func TestAgentContract_ContextCancellation(t *testing.T) {
    agent := setupAgentImplementation(t)
    ctx, cancel := context.WithCancel(context.Background())

    iter, err := agent.SendMessage(ctx, testChat, testText)
    require.NoError(t, err)

    // Cancel after first chunk
    firstChunk := true
    for _, err := range iter {
        if firstChunk {
            cancel()
            firstChunk = false
        }
        // Should complete shortly after cancellation
    }

    // Verify cleanup (no leaked goroutines)
    time.Sleep(100 * time.Millisecond)
    assert.Equal(t, startingGoroutines, runtime.NumGoroutine())
}
```

### Test 4: Error Handling

```go
func TestAgentContract_A2AUnavailable(t *testing.T) {
    agent := setupAgentWithUnavailableServer(t)
    ctx := context.Background()

    iter, err := agent.SendMessage(ctx, testChat, testText)

    // Should return error immediately or yield error in first iteration
    if err != nil {
        assert.Error(t, err)
    } else {
        _, err := iter(func(components.MessageText, error) bool { return true })
        assert.Error(t, err)
    }
}
```

### Test 5: Invalid Input

```go
func TestAgentContract_InvalidInput(t *testing.T) {
    agent := setupAgentImplementation(t)
    ctx := context.Background()

    tests := []struct {
        name string
        chat ids.MessageID
        text components.MessageText
    }{
        {"invalid chat", ids.MessageID{}, validText},
        {"invalid text", validChat, components.MessageText{}},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := agent.SendMessage(ctx, tt.chat, tt.text)
            assert.Error(t, err)
        })
    }
}
```

---

## Implementation Checklist

When implementing the `Agent` port:

- [ ] Validate inputs (chat and text) before making A2A request
- [ ] Map domain types to A2A protocol correctly (message_id, context_id)
- [ ] Use gRPC `SendStreamingMessage` for streaming support
- [ ] Parse A2A Part messages to extract text content (not `.String()`)
- [ ] Yield chunks incrementally via iterator
- [ ] Handle `io.EOF` to complete iterator gracefully
- [ ] Propagate context cancellation through gRPC stream
- [ ] Close gRPC stream in iterator cleanup
- [ ] Wrap errors with context for debugging
- [ ] Add structured logging for observability (optional)
- [ ] Write contract tests to verify compliance

---

## Example Implementation Snippets

### Correct Protobuf Parsing

```go
func extractTextFromA2AMessage(msg *a2a.Message) (string, error) {
    var text strings.Builder
    for _, part := range msg.GetContent() {
        switch p := part.GetPart().(type) {
        case *a2a.Part_Text:
            text.WriteString(p.Text)
        case *a2a.Part_ToolCall:
            // Skip for MVP (out of scope)
        }
    }
    if text.Len() == 0 {
        return "", errors.New("no text content in message")
    }
    return text.String(), nil
}
```

### Iterator with Proper Cleanup

```go
return func(yield func(components.MessageText, error) bool) {
    defer stream.CloseSend() // Cleanup gRPC stream

    for {
        select {
        case <-ctx.Done():
            yield(components.MessageText{}, ctx.Err())
            return
        default:
            resp, err := stream.Recv()
            if errors.Is(err, io.EOF) {
                return
            }
            if err != nil {
                yield(components.MessageText{}, err)
                return
            }

            text, err := extractTextFromA2AMessage(resp.GetMsg())
            if err != nil {
                yield(components.MessageText{}, err)
                return
            }

            msgText, _ := components.NewMessageText(text)
            if !yield(msgText, nil) {
                return
            }
        }
    }
}, nil
```

---

## References

- A2A Protocol Specification: `google.golang.org/a2a`
- Go Iterator Pattern: `iter.Seq2` documentation
- gRPC Streaming: `grpc.io/docs/languages/go/basics/#server-side-streaming-rpc`

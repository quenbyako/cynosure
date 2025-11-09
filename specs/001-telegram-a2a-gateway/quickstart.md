# Quickstart: Telegram-to-A2A Gateway Development

**Feature**: 001-telegram-a2a-gateway
**Branch**: `001-telegram-a2a-gateway`
**For**: Developers implementing or testing the gateway

---

## Prerequisites

- Go 1.21+ installed
- Telegram bot token (obtain from [@BotFather](https://t.me/botfather))
- A2A server running locally or accessible endpoint
- ngrok or similar tool for webhook testing (optional, for local development)

---

## Quick Setup

### 1. Clone and Switch to Feature Branch

```bash
cd /Users/rcooper/repositories/tg-helper
git checkout 001-telegram-a2a-gateway
```

### 2. Configure Environment

Create or update `secrets.env` with:

```bash
# Telegram Bot Configuration
TELEGRAM_BOT_TOKEN="your_bot_token_here"
TELEGRAM_WEBHOOK_ADDR="https://your-domain.com/webhook"  # Or ngrok URL

# A2A Server Configuration
A2A_SERVER_ADDR="localhost:5001"  # Your A2A server address

# HTTP Server Configuration
HTTP_PORT="8080"
```

### 3. Install Dependencies

```bash
go mod download
go mod tidy
```

### 4. Build the Gateway

```bash
go build -o bin/cynosure ./cmd/cynosure
```

---

## Running Locally

### Option 1: With Real Telegram Bot (requires webhook)

1. **Start ngrok** (for local webhook testing):
   ```bash
   ngrok http 8080
   ```
   Copy the HTTPS URL (e.g., `https://abc123.ngrok.io`)

2. **Update webhook address** in `secrets.env`:
   ```bash
   TELEGRAM_WEBHOOK_ADDR="https://abc123.ngrok.io/webhook"
   ```

3. **Start A2A server** (if not already running):
   ```bash
   # In cynosure main app terminal
   ./bin/cynosure cynosure
   ```

4. **Start gateway**:
   ```bash
   ./bin/cynosure gateway
   ```

5. **Test**: Send a message to your Telegram bot

### Option 2: With Mock A2A Server (for development)

```bash
# Terminal 1: Start mock A2A server
go run ./tests/mocks/a2a-server/main.go

# Terminal 2: Start gateway
A2A_SERVER_ADDR="localhost:9001" ./bin/cynosure gateway
```

---

## Development Workflow

### Step 1: Fix Concurrency Bug

**File**: `internal/domains/gateway/usecases/usecase.go`

**Current Issue** (line 52):
```go
wg.Go(func() {  // ❌ Wrong: wg.Go() doesn't exist
```

**Fix**:
```go
go func() {
    defer close(textChan)  // ✅ Close channel when done

    for part, err := range resp {
        if err != nil {
            // Handle error
            continue
        }
        select {
        case textChan <- part:
        case <-ctx.Done():
            return
        }
    }
}()
```

**Test**:
```bash
go test ./internal/domains/gateway/usecases -v -run TestUsecase_ReceiveNewMessageEvent
```

### Step 2: Fix A2A Response Parsing

**File**: `internal/adapters/a2a/client.go`

**Current Issue** (line 89):
```go
msg, err := components.NewMessageText(resp.GetMsg().String())  // ❌ Wrong: .String() is debug format
```

**Fix**:
```go
// Extract text from protobuf Part messages
var text strings.Builder
for _, part := range resp.GetMsg().GetContent() {
    switch p := part.GetPart().(type) {
    case *a2a.Part_Text:
        text.WriteString(p.Text)
    }
}

if text.Len() == 0 {
    yield(components.MessageText{}, errors.New("no text content"))
    return
}

msg, err := components.NewMessageText(text.String())
```

**Test**:
```bash
go test ./internal/adapters/a2a -v -run TestA2AClient_ParsesProtobufCorrectly
```

### Step 3: Implement Streaming in Telegram Adapter

**File**: `internal/adapters/telegram/client.go`

**Current Issue**: Accumulates all text then sends once (lines 65-91)

**Implementation**:
1. Send initial message with first chunk
2. Use `time.Ticker` for 3-second update intervals
3. Edit message with accumulated text
4. Send final update when channel closes

**Refer to**: `specs/001-telegram-a2a-gateway/contracts/messenger-port.md` for detailed requirements

**Test**:
```bash
go test ./internal/adapters/telegram -v -run TestTelegramAdapter_Streaming
```

### Step 4: Add Error Handling

**File**: `internal/domains/gateway/usecases/usecase.go`

**Add helper function**:
```go
func userFriendlyError(err error) string {
    if errors.Is(err, context.DeadlineExceeded) {
        return "⏱ The agent is taking too long to respond. Please try again."
    }
    if status.Code(err) == codes.Unavailable {
        return "🔌 The agent service is temporarily unavailable. Please try again in a few moments."
    }
    return "❌ An unexpected error occurred. Please try again."
}
```

**Update usecase** to send errors to users:
```go
if err := u.a2a.SendMessage(...); err != nil {
    // Send user-friendly error
    friendlyMsg := userFriendlyError(err)
    u.client.SendMessage(ctx, channelID, makeSingleTextChan(friendlyMsg))
    return fmt.Errorf("a2a error: %w", err)
}
```

### Step 5: Add Structured Logging

**File**: `internal/controllers/tgbot/controller.go`

**Replace** `pp.Println` with structured logging:
```go
h.log.ProcessMessageStart(ctx, chatID, messageID)
// ... processing ...
h.log.ProcessMessageSuccess(ctx, chatID, messageID, time.Since(start))
```

---

## Testing

### Run All Tests

```bash
# Unit tests
go test ./internal/... -v

# Contract tests
go test ./tests/contract/... -v

# Integration tests (requires mock servers)
go test ./tests/integration/... -v
```

### Run Specific Test Suites

```bash
# Concurrency tests
go test ./internal/domains/gateway/usecases -v -run Concurrency

# Streaming tests
go test ./internal/adapters/telegram -v -run Streaming

# Parsing tests
go test ./internal/adapters/a2a -v -run Parsing
```

### Test Coverage

```bash
go test ./internal/... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

**Target**: >80% coverage for changed files

---

## Manual Testing Scenarios

### Scenario 1: Basic Message Exchange

1. Start gateway with A2A server running
2. Send message to Telegram bot: "Hello"
3. **Expected**: Bot responds with agent's message within 5 seconds
4. **Verify**: Typing indicator appears immediately

### Scenario 2: Long Streaming Response

1. Configure A2A to return long response (>500 characters)
2. Send message: "Tell me a long story"
3. **Expected**: Message updates progressively every ~3 seconds
4. **Verify**: Message edits visible in Telegram, final message complete

### Scenario 3: A2A Server Unavailable

1. Stop A2A server
2. Send message to bot: "Test"
3. **Expected**: Bot responds with error message: "🔌 The agent service is temporarily unavailable..."
4. **Verify**: Error message appears within 10 seconds

### Scenario 4: Very Long Response (>4096 chars)

1. Configure A2A to return 5000 character response
2. Send message to bot
3. **Expected**: Message truncated to 4096 chars with "...[truncated]" indicator
4. **Verify**: No errors, message visible in Telegram

### Scenario 5: Concurrent Users

1. Have 5+ users send messages simultaneously
2. **Expected**: All users receive responses without interference
3. **Verify**: No goroutine leaks (check with `pprof`)

---

## Debugging

### Enable Debug Logging

```bash
export LOG_LEVEL=debug
./bin/cynosure gateway
```

### Check Goroutine Leaks

```bash
# Add pprof endpoint (already included in codebase)
curl http://localhost:8080/debug/pprof/goroutine?debug=1
```

### Telegram Webhook Issues

```bash
# Check webhook status
curl "https://api.telegram.org/bot<YOUR_BOT_TOKEN>/getWebhookInfo"

# Delete webhook (for testing)
curl "https://api.telegram.org/bot<YOUR_BOT_TOKEN>/deleteWebhook"
```

### A2A Connection Issues

```bash
# Test gRPC connection
grpcurl -plaintext localhost:5001 list
grpcurl -plaintext localhost:5001 a2a.A2AService/SendMessage
```

---

## Common Issues

### Issue: "wg.Go() undefined"

**Cause**: Incorrect WaitGroup usage
**Fix**: Use `go func()` with `defer close(textChan)`
**See**: Step 1 in Development Workflow

### Issue: Response shows debug format like "role:ROLE_AGENT content:..."

**Cause**: Using `.String()` on protobuf message
**Fix**: Parse `Part.Text` fields correctly
**See**: Step 2 in Development Workflow

### Issue: Message never updates, shows all at once

**Cause**: Streaming not implemented in Telegram adapter
**Fix**: Implement time-based batching with ticker
**See**: Step 3 in Development Workflow

### Issue: Bot silent on errors

**Cause**: Errors suppressed in usecase
**Fix**: Add user-friendly error notifications
**See**: Step 4 in Development Workflow

### Issue: Webhook returns 500 errors

**Check**:
1. Logs in terminal (structured logging should show errors)
2. Webhook payload format (Telegram sends specific protobuf structure)
3. Port mapping (ngrok → localhost:8080)

---

## Architecture Reference

### Key Files to Understand

| File | Purpose |
|------|---------|
| `internal/domains/gateway/usecases/usecase.go` | Core orchestration logic |
| `internal/adapters/telegram/client.go` | Telegram Bot API integration |
| `internal/adapters/a2a/client.go` | A2A gRPC client |
| `internal/controllers/tgbot/controller.go` | Webhook HTTP handler |
| `cmd/cynosure/root/gateway/cmd.go` | Gateway app entrypoint |

### Data Flow Diagram

```
┌─────────────┐
│ Telegram    │
│ User        │
└──────┬──────┘
       │ 1. Send message
       ↓
┌─────────────────────┐
│ TgBot Controller    │ (HTTP webhook handler)
│ Parse protobuf      │
└──────┬──────────────┘
       │ 2. Create Message entity
       ↓
┌─────────────────────┐
│ Gateway Usecase     │ (Domain orchestration)
│ - Show typing       │
│ - Forward to A2A    │
│ - Stream back       │
└──────┬──────────────┘
       │ 3. Forward message
       ↓
┌─────────────────────┐
│ A2A Adapter         │ (gRPC client)
│ Parse responses     │
└──────┬──────────────┘
       │ 4. Stream response
       ↓
┌─────────────────────┐
│ A2A Server          │ (Cynosure agent runtime)
│ Generate response   │
└──────┬──────────────┘
       │ 5. Yield chunks
       ↓
┌─────────────────────┐
│ Telegram Adapter    │ (Bot API client)
│ Batch updates       │
└──────┬──────────────┘
       │ 6. Edit message
       ↓
┌─────────────┐
│ Telegram    │
│ User        │
└─────────────┘
```

---

## Next Steps After Implementation

1. **Run full test suite**: Ensure all tests pass
2. **Manual testing**: Test all scenarios above
3. **Performance testing**: Verify concurrent users work smoothly
4. **Code review**: Submit PR with constitution compliance checklist
5. **Deployment**: Deploy to staging environment with real Telegram bot
6. **Monitoring**: Set up metrics and alerts for error rates, latency

---

## References

- **Specification**: `specs/001-telegram-a2a-gateway/spec.md`
- **Research**: `specs/001-telegram-a2a-gateway/research.md`
- **Data Model**: `specs/001-telegram-a2a-gateway/data-model.md`
- **Port Contracts**: `specs/001-telegram-a2a-gateway/contracts/`
- **Telegram Bot API**: https://core.telegram.org/bots/api
- **A2A Protocol**: `google.golang.org/a2a`
- **Go Concurrency**: https://go.dev/blog/pipelines

---

## Getting Help

- **Constitution questions**: Review `.specify/memory/constitution.md`
- **Port contract questions**: See `contracts/` directory
- **Architecture questions**: Refer to data model and plan documents
- **Testing questions**: See contract test examples in `contracts/` files

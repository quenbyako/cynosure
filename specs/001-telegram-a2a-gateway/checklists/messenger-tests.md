# Messenger Port Test Cases Checklist

**Port**: `internal/domains/gateway/ports/Messenger`
**Test Suite**: `internal/domains/gateway/ports/testsuite/messenger.go`
**Reference**: Based on `cynosure/ports/testsuite/chat_model.go` pattern

## Port Interface

```go
type Messenger interface {
    SendMessage(ctx context.Context, channelID ids.ChannelID, text components.MessageText) (ids.MessageID, error)
    UpdateMessage(ctx context.Context, messageID ids.MessageID, text components.MessageText) error
    NotifyProcessingStarted(ctx context.Context, channelID ids.ChannelID) error
}
```

---

## Test Categories

### 1. Basic Functionality Tests

#### SendMessage Tests
- [ ] **TestSendMessage_BasicText** - Send simple text message, verify returns valid MessageID
- [ ] **TestSendMessage_EmptyText** - Send empty text, verify returns error
- [ ] **TestSendMessage_InvalidChannelID** - Send to invalid channel, verify returns error
- [ ] **TestSendMessage_ContextCancellation** - Cancel context during send, verify proper cancellation
- [ ] **TestSendMessage_ReturnsCorrectMessageID** - Verify returned MessageID matches channel and provider

#### UpdateMessage Tests
- [ ] **TestUpdateMessage_BasicUpdate** - Update existing message with new text, verify success
- [ ] **TestUpdateMessage_InvalidMessageID** - Update with invalid message ID, verify returns error
- [ ] **TestUpdateMessage_EmptyText** - Update with empty text, verify behavior (should skip or error)
- [ ] **TestUpdateMessage_NotModified** - Update with same text, verify ignores "not modified" errors
- [ ] **TestUpdateMessage_NonExistentMessage** - Update message that doesn't exist, verify returns error
- [ ] **TestUpdateMessage_ContextCancellation** - Cancel context during update, verify proper cancellation

#### NotifyProcessingStarted Tests
- [ ] **TestNotifyProcessingStarted_ValidChannel** - Notify valid channel, verify success
- [ ] **TestNotifyProcessingStarted_InvalidChannel** - Notify invalid channel, verify returns error
- [ ] **TestNotifyProcessingStarted_MultipleNotifications** - Send multiple notifications, verify all succeed

---

### 2. Message Length & Truncation Tests

- [ ] **TestSendMessage_ShortMessage** - Send message < 100 chars, verify sent as-is
- [ ] **TestSendMessage_MediumMessage** - Send message ~2000 chars, verify sent successfully
- [ ] **TestSendMessage_LongMessage** - Send message > 4096 chars, verify truncates at 4080 with "...[truncated]"
- [ ] **TestSendMessage_ExactlyAtLimit** - Send message exactly 4096 chars, verify behavior
- [ ] **TestSendMessage_UnicodeCharacters** - Send message with emoji/unicode, verify correct length calculation
- [ ] **TestUpdateMessage_Truncation** - Update with very long text, verify truncates properly

---

### 3. Provider Validation Tests

- [ ] **TestSendMessage_TelegramProvider** - Send to "telegram" provider, verify success
- [ ] **TestSendMessage_UnsupportedProvider** - Send to non-telegram provider, verify returns error
- [ ] **TestUpdateMessage_TelegramProvider** - Update telegram message, verify success
- [ ] **TestUpdateMessage_UnsupportedProvider** - Update non-telegram message, verify returns error

---

### 4. Streaming & Batching Tests (User Story 2)

- [ ] **TestSendMessage_FirstChunkImmediate** - Verify first message sent immediately (no batching delay)
- [ ] **TestUpdateMessage_BatchingBehavior** - Verify updates respect time-based batching
- [ ] **TestUpdateMessage_FinalUpdate** - Verify final update sent when stream completes
- [ ] **TestUpdateMessage_RapidUpdates** - Send many rapid updates, verify proper throttling

---

### 5. Error Handling Tests (User Story 3)

#### Network & API Errors
- [ ] **TestSendMessage_NetworkTimeout** - Simulate network timeout, verify returns error
- [ ] **TestSendMessage_APIError** - Simulate messenger API error, verify returns error
- [ ] **TestUpdateMessage_NetworkFailure** - Simulate network failure during update, verify error handling
- [ ] **TestUpdateMessage_MessageDeleted** - Update deleted message, verify appropriate error

#### Rate Limiting
- [ ] **TestSendMessage_RateLimit** - Simulate rate limit, verify returns error
- [ ] **TestUpdateMessage_RateLimit** - Simulate rate limit on updates, verify backoff/retry behavior
- [ ] **TestMessenger_RateLimitCompliance** - Verify adapter respects platform rate limits (20 msg/sec)

---

### 6. Context & Concurrency Tests

- [ ] **TestSendMessage_ContextDeadline** - Set short deadline, verify timeout error
- [ ] **TestSendMessage_ConcurrentCalls** - Send messages concurrently, verify all succeed
- [ ] **TestUpdateMessage_ConcurrentUpdates** - Update different messages concurrently, verify thread-safety
- [ ] **TestUpdateMessage_SameMessageConcurrent** - Update same message concurrently, verify no race conditions
- [ ] **TestMessenger_GoroutineLeaks** - Verify no goroutine leaks after operations

---

### 7. Integration Scenarios

#### Full Message Lifecycle
- [ ] **TestMessenger_FullLifecycle** - NotifyProcessing → SendMessage → UpdateMessage sequence
- [ ] **TestMessenger_MultipleUpdates** - Send message, update multiple times, verify final state
- [ ] **TestMessenger_InterleavedOperations** - Mix send, update, notify across multiple channels

#### Multi-Channel Tests
- [ ] **TestMessenger_MultipleChannels** - Send to multiple channels simultaneously, verify isolation
- [ ] **TestMessenger_ChannelSwitching** - Switch between channels rapidly, verify correct routing
- [ ] **TestMessenger_ChannelIsolation** - Verify message IDs don't cross channels

---

### 8. Edge Cases & Boundary Conditions

#### Text Content
- [ ] **TestSendMessage_Whitespace** - Send message with only whitespace, verify behavior
- [ ] **TestSendMessage_SpecialCharacters** - Send message with special chars (markdown, HTML), verify escaping
- [ ] **TestSendMessage_Newlines** - Send message with newlines, verify formatting preserved
- [ ] **TestSendMessage_CodeBlocks** - Send message with code blocks, verify formatting

#### Message IDs
- [ ] **TestUpdateMessage_WrongChannelID** - Update message with wrong channel ID, verify error
- [ ] **TestUpdateMessage_MalformedMessageID** - Use malformed message ID, verify error
- [ ] **TestSendMessage_MessageIDFormat** - Verify returned message ID follows expected format

#### Resource Cleanup
- [ ] **TestMessenger_Cleanup** - Verify resources cleaned up after operations
- [ ] **TestMessenger_RepeatedOperations** - Perform operations repeatedly, verify no resource exhaustion

---

### 9. Performance Tests

- [ ] **TestSendMessage_Latency** - Verify message sending latency < 500ms
- [ ] **TestUpdateMessage_Latency** - Verify message update latency < 500ms
- [ ] **TestMessenger_Throughput** - Verify can handle 10+ concurrent conversations
- [ ] **TestMessenger_MemoryUsage** - Verify reasonable memory consumption under load

---

### 10. Compliance & Validation Tests

#### Telegram Limits (if Telegram adapter)
- [ ] **TestTelegram_MessageLengthLimit** - Verify 4096 char limit enforcement
- [ ] **TestTelegram_EditTimeLimit** - Verify handles edit time restrictions (48 hours)
- [ ] **TestTelegram_ChatIDFormat** - Verify correct chat ID format (numeric)
- [ ] **TestTelegram_BotAPICompliance** - Verify uses correct Bot API endpoints

#### Port Contract
- [ ] **TestMessenger_InterfaceCompliance** - Verify implements full Messenger interface
- [ ] **TestMessenger_ErrorTypes** - Verify returns appropriate error types
- [ ] **TestMessenger_NilHandling** - Verify handles nil parameters gracefully

---

## Test Implementation Priority

### P0 - Critical (Must Have)
1. TestSendMessage_BasicText
2. TestUpdateMessage_BasicUpdate
3. TestNotifyProcessingStarted_ValidChannel
4. TestSendMessage_LongMessage (truncation)
5. TestMessenger_FullLifecycle

### P1 - Important (Should Have)
6. TestSendMessage_InvalidChannelID
7. TestUpdateMessage_NotModified
8. TestSendMessage_TelegramProvider
9. TestUpdateMessage_BatchingBehavior
10. TestMessenger_MultipleChannels

### P2 - Nice to Have
11. All error handling tests (T050-T052)
12. All concurrency tests
13. All edge cases
14. Performance tests

---

## Test Data Requirements

### Mock Channel IDs
```go
validTelegramChannel := mustChannelID("telegram", "123456789")
invalidChannel := mustChannelID("invalid", "bad-id")
```

### Mock Message Text
```go
shortText := mustMessageText("Hello")
mediumText := mustMessageText(strings.Repeat("A", 2000))
longText := mustMessageText(strings.Repeat("B", 5000))
```

### Mock Message IDs
```go
validMessageID := mustMessageID(validTelegramChannel, "1")
invalidMessageID := ids.MessageID{} // empty/invalid
```

---

## Test Suite Structure

```go
type MessengerTestSuite struct {
    adapter ports.Messenger
    timeout time.Duration  // optional: custom timeout
    skipSlow bool          // optional: skip performance tests
}

func (s *MessengerTestSuite) TestSendMessage_BasicText(t *testing.T) {
    // Arrange
    channelID := mustChannelID("telegram", "123456")
    text := mustMessageText("Test message")

    // Act
    messageID, err := s.adapter.SendMessage(t.Context(), channelID, text)

    // Assert
    require.NoError(t, err, "SendMessage should succeed")
    require.True(t, messageID.Valid(), "Should return valid message ID")
    require.Equal(t, channelID, messageID.ChannelID(), "Message ID should reference correct channel")
}

// ... more tests following same pattern
```

---

## Implementation Notes

1. **Follow bettersuites pattern**: Use `suites.Run(s)` for test execution
2. **Use must() helper**: For test data setup that shouldn't fail
3. **Context usage**: Always use `t.Context()` for proper cancellation
4. **Assertions**: Use `require` for critical checks, `assert` for non-critical
5. **Test isolation**: Each test should be independent, no shared state
6. **Cleanup**: Use `t.Cleanup()` for resource cleanup when needed

---

## Coverage Goal

- **Line Coverage**: > 80% for adapter implementations
- **Branch Coverage**: > 70% for error paths
- **Integration Coverage**: All port methods exercised in realistic scenarios

---

## Related Tasks

- T021: TestMessengerContract_BasicSend
- T022: TestMessengerContract_NotifyProcessing
- T035: TestMessengerContract_Streaming
- T036: TestMessengerContract_RateLimitingCompliance
- T051: TestMessengerContract_ErrorMessageSend

**Total Test Cases**: 62
**Estimated Implementation Time**: 4-6 hours
**Priority**: P1 (Important for production readiness)

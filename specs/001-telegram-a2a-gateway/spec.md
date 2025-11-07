# Feature Specification: Complete Telegram-to-A2A Gateway with Streaming Support

**Feature Branch**: `001-telegram-a2a-gateway`
**Created**: 2025-11-07
**Status**: Draft
**Input**: Complete implementation of Telegram bot gateway that proxies messages to A2A server with streaming support

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Basic Message Exchange (Priority: P1)

A user sends a text message to the Telegram bot, the bot forwards it to the A2A server, receives a response, and displays it back to the user in the Telegram chat.

**Why this priority**: This is the core functionality - without this, the gateway provides no value. It's the foundation for all other features.

**Independent Test**: Can be fully tested by sending a text message to the bot and verifying a response is received from the A2A agent. Delivers immediate value by enabling basic agent interaction through Telegram.

**Acceptance Scenarios**:

1. **Given** a user has access to the Telegram bot, **When** they send a text message "Hello", **Then** the bot forwards the message to the A2A server and displays the agent's response in the chat
2. **Given** the A2A server is processing a message, **When** user waits, **Then** the bot shows a "typing" indicator to provide feedback that processing is in progress
3. **Given** a message contains only emojis or special characters, **When** the message is sent, **Then** the bot correctly forwards the content to the A2A server without corruption

---

### User Story 2 - Streaming Response Updates (Priority: P2)

When the A2A server sends a long streaming response, the user sees the message progressively update in real-time (or near real-time at ~3 second intervals) rather than waiting for the complete response.

**Why this priority**: Provides better user experience for long-running agent responses, reduces perceived latency, and gives users confidence the system is working.

**Independent Test**: Can be tested by triggering a long agent response and verifying that the Telegram message updates progressively rather than appearing all at once. Delivers value by improving UX without changing core functionality.

**Acceptance Scenarios**:

1. **Given** the A2A server sends a streaming response, **When** new content arrives, **Then** the bot updates the existing Telegram message approximately every 3 seconds with accumulated content
2. **Given** a streaming response is in progress, **When** the stream completes, **Then** the final message displays the complete response without truncation
3. **Given** multiple message chunks arrive within a 3-second window, **When** the update timer triggers, **Then** all accumulated chunks are included in the single message edit

---

### User Story 3 - Error Recovery and User Notification (Priority: P2)

When errors occur during message processing (A2A server unavailable, network issues, parsing errors), the user receives clear feedback about what went wrong instead of seeing nothing or experiencing a timeout.

**Why this priority**: Essential for production reliability and user trust. Without proper error handling, users don't know if the bot is broken or just slow.

**Independent Test**: Can be tested by simulating various failure scenarios (disconnect A2A server, send malformed data) and verifying appropriate error messages are sent to users. Delivers value by making the system more reliable and user-friendly.

**Acceptance Scenarios**:

1. **Given** the A2A server is unavailable, **When** a user sends a message, **Then** the bot responds with a clear error message like "Agent service is currently unavailable, please try again later"
2. **Given** the A2A server returns an error during processing, **When** the error occurs, **Then** the bot notifies the user with "An error occurred while processing your message" rather than leaving them waiting
3. **Given** a network timeout occurs, **When** the timeout threshold is exceeded, **Then** the user receives a timeout notification instead of indefinite waiting

---

### User Story 4 - Context Preservation Across Messages (Priority: P3)

Users can have multi-turn conversations where the A2A agent maintains context of previous messages within the chat session.

**Why this priority**: Enables natural conversations but requires A2A server support for context management. Can be deferred if basic message exchange is the MVP.

**Independent Test**: Can be tested by sending a sequence of related messages and verifying the agent's responses demonstrate awareness of prior context. Delivers value by enabling conversation continuity.

**Acceptance Scenarios**:

1. **Given** a user has sent previous messages in a chat, **When** they send a follow-up message, **Then** the A2A server receives the chat context identifier to maintain conversation continuity
2. **Given** a user switches to a different chat, **When** they send a message, **Then** each chat maintains separate conversation context
3. **Given** the A2A server tracks context by context_id, **When** the gateway sends messages, **Then** it consistently uses the Telegram chat ID as the context_id

---

### Edge Cases

- What happens when a Telegram message is deleted or edited after being sent to A2A?
  *Initial implementation: Ignore edits and deletions, treat each message as immutable once sent*

- How does the system handle messages arriving faster than A2A can process?
  *Initial implementation: Process messages sequentially per chat to maintain order, queue if necessary*

- What happens when a streaming response takes longer than Telegram's message edit rate limits allow?
  *Implementation: Respect Telegram API rate limits, batch updates to avoid hitting limits, may result in slower than 3-second updates if necessary*

- How does the system handle very long responses that exceed Telegram's message length limits (4096 characters)?
  *Initial implementation: Truncate at 4090 characters with "...[truncated]" suffix (T033). Future enhancement may implement multi-message splitting.*

- What happens when the bot restarts during active message processing?
  *Initial implementation: In-flight messages are lost, users must resend. Future: Implement persistence/recovery*

- How are non-text messages (photos, files, voice) handled?
  *Explicitly out of scope for first iteration as specified in requirements*

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST receive text messages from Telegram bot webhook and forward them to the configured A2A server endpoint
- **FR-002**: System MUST send A2A agent responses back to the originating Telegram chat
- **FR-003**: System MUST display "typing" indicator in Telegram when message processing begins
- **FR-004**: System MUST support A2A streaming responses by progressively updating the Telegram message
- **FR-005**: System MUST update streaming responses approximately every 3 seconds (within Telegram API rate limits)
- **FR-006**: System MUST handle errors gracefully and notify users when message processing fails
- **FR-007**: System MUST use Telegram chat ID as the A2A context_id to enable conversation continuity
- **FR-008**: System MUST preserve message ordering within a single chat session
- **FR-009**: System MUST be fully compatible with A2A protocol specification (no custom extensions)
- **FR-010**: System MUST properly close streaming connections when responses complete
- **FR-011**: System MUST handle concurrent messages from different users/chats without interference
- **FR-012**: System MUST validate message content before forwarding to prevent empty or invalid requests

### Non-Functional Requirements

- **NFR-001**: System MUST handle at least 10 concurrent conversations without degradation (defined as: <10% latency increase and no error rate increase compared to single conversation baseline). Verified via concurrent test T070.
- **NFR-002**: Message forwarding latency MUST be under 500ms for 95th percentile (measured from webhook receipt to A2A SendMessage call, excluding A2A processing time). Verified via metrics added in tasks T072-T074.
- **NFR-003**: System MUST properly clean up goroutines and connections to prevent resource leaks
- **NFR-004**: System MUST respect Telegram Bot API rate limits to avoid being throttled

### Explicitly Out of Scope (First Iteration)

- Multimedia message support (images, video, audio)
- Group chat support (only private messages)
- Message editing after sending to A2A
- Authentication beyond basic Telegram user identification
- Persistent conversation history storage
- Support for A2A tool/function calls with user approval flows

### Key Entities

- **Message**: Represents a user message from Telegram with text content, sender information, and chat context. Maps to A2A Message structure with contextId and messageId.
- **Channel**: Identifies the Telegram chat where messages are sent/received. Used as A2A context_id for conversation continuity.
- **Stream**: Represents an ongoing A2A streaming response, accumulating message chunks until completion.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can send a message to the bot and receive a response within 5 seconds (for simple A2A responses)
- **SC-002**: Streaming responses visibly update in the Telegram chat at intervals of approximately 3-5 seconds
- **SC-003**: System successfully handles 10 concurrent conversations without errors or timeouts
- **SC-004**: When A2A server is unavailable, users receive an error notification within 10 seconds rather than timing out silently
- **SC-005**: 95% of messages are successfully forwarded to A2A and responses returned without errors
- **SC-006**: Zero goroutine leaks during normal operation (verified by runtime metrics)
- **SC-007**: System can run continuously for 24 hours without memory leaks or resource exhaustion
- **SC-008**: Multi-turn conversations maintain context correctly (verified by A2A context_id matching Telegram chat_id)

### User Experience Criteria

- Users see "typing" indicator immediately after sending message
- Long responses appear progressively rather than all at once
- Error messages are clear and actionable
- No "ghost" messages or partial responses from failed requests

## Dependencies & Assumptions

### Dependencies
- Telegram Bot API (webhook mode)
- A2A server endpoint availability
- gRPC for A2A communication
- Existing bot token and webhook configuration

### Assumptions
- A2A server supports streaming responses via `SendStreamingMessage`
- A2A server uses `context_id` for conversation continuity
- Telegram webhook is properly configured and receives updates
- Bot has necessary permissions in target chats
- Network connectivity between gateway and A2A server is stable

### Integration Points
- **Telegram Bot API**: Webhook for receiving updates, REST API for sending messages
- **A2A Protocol**: gRPC streaming interface as defined in A2A specification
- **Configuration**: Environment variables for bot token, A2A endpoint, webhook URL

## Technical Context (for implementation planning)

### Current Implementation Status

The gateway has partial implementation with these components:

- **A2A Controller** (`internal/controllers/a2a/handler.go`): Handles incoming A2A protocol requests
- **Telegram Adapter** (`internal/adapters/telegram/client.go`): Telegram Bot API integration
- **A2A Client Adapter** (`internal/adapters/a2a/client.go`): Client for connecting to A2A servers
- **Gateway Usecase** (`internal/domains/gateway/usecases/usecase.go`): Orchestrates message flow
- **Telegram Controller** (`internal/controllers/tgbot/controller.go`): Receives webhook updates

### Critical Issues to Fix

1. **Concurrency Bug** (`internal/domains/gateway/usecases/usecase.go:52`):
   - Uses non-existent `wg.Go()` method on `sync.WaitGroup`
   - Channel never closed, causing goroutine leaks
   - `SendMessage` called before goroutine finishes

2. **No Streaming Implementation** (`internal/adapters/telegram/client.go:65-91`):
   - Accumulates all text then sends once
   - Missing progressive message update logic
   - No message editing for streaming responses

3. **Incorrect A2A Response Parsing** (`internal/adapters/a2a/client.go:89`):
   - Uses `.String()` debug representation instead of proper protobuf parsing
   - Should extract text from Part messages correctly

4. **Suppressed Error Handling** (`internal/domains/gateway/usecases/usecase.go:57`):
   - Errors ignored with TODO comments
   - No user notification on failures

### Architecture Pattern

Ports-and-adapters (hexagonal) architecture:
- **Domain** (`internal/domains/gateway`): Core business logic
- **Ports** (`internal/domains/gateway/ports`): Interfaces for external dependencies
- **Adapters** (`internal/adapters`): Concrete implementations (Telegram, A2A)
- **Controllers** (`internal/controllers`): HTTP/gRPC request handlers

### Data Flow

```
Telegram User → Webhook → TgBot Controller → Gateway Usecase → A2A Client → A2A Server
                                                                                ↓
Telegram User ← Telegram Adapter ← Gateway Usecase ← Stream Iterator ← A2A Server
```

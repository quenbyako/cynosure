# Data Model: Telegram-to-A2A Gateway

**Feature**: 001-telegram-a2a-gateway
**Phase**: 1 - Design
**Date**: 2025-11-07

## Overview

This document describes the domain model for the gateway bounded context. The gateway is a stateless proxy, so entities represent transient data from webhooks and API responses rather than persistent aggregates.

---

## Domain Entities

### 1. Message (Entity)

**Purpose**: Represents a user message received from Telegram webhook.

**Location**: `internal/domains/gateway/entities/message.go`

**Attributes**:
- `id` (MessageID): Unique identifier combining channel and message sequence number
- `author` (UserID): User who sent the message
- `text` (MessageText): Optional text content
- `timestamp` (time.Time): When message was received

**Invariants**:
- Message ID must be valid (non-empty channel and message components)
- At least one content type must be present (text for MVP, extensible for future media)
- Author ID must be valid

**Relationships**:
- Belongs to exactly one Channel (via MessageID.ChannelID())
- Sent by exactly one User (author)

**State Transitions**: None (immutable after creation from webhook)

**Validation Rules**:
```go
func NewMessage(id MessageID, author UserID, opts ...Option) (*Message, error)
```
- Validates IDs are non-empty
- Ensures at least text content provided for MVP
- Returns validation error if constraints violated

---

## Value Objects

### 2. MessageText (Value Object)

**Purpose**: Encapsulates message text with validation.

**Location**: `internal/domains/gateway/components/message_text.go`

**Attributes**:
- `text` (string): UTF-8 encoded text content

**Invariants**:
- Text cannot be empty (after trimming whitespace)
- Text length ≤ 4096 characters (Telegram limit)
- Valid UTF-8 encoding

**Operations**:
```go
func NewMessageText(text string) (MessageText, error)
func (m MessageText) Text() string
func (m MessageText) Valid() bool
```

**Validation**:
- Trims leading/trailing whitespace
- Rejects empty strings
- Enforces character limit
- Validates UTF-8 encoding

---

### 3. MessageID (Value Object)

**Purpose**: Unique identifier for a message within a channel.

**Location**: `internal/domains/gateway/components/ids/message_id.go`

**Structure**:
```
<provider>:<channel_id>:<message_sequence>
Example: telegram:123456789:42
```

**Attributes**:
- `channelID` (ChannelID): Channel where message was sent
- `messageSeq` (string): Platform-specific message identifier

**Invariants**:
- Channel ID must be valid
- Message sequence must be non-empty
- String representation must be parseable

**Operations**:
```go
func NewMessageID(channelID ChannelID, seq string) (MessageID, error)
func (m MessageID) ChannelID() ChannelID
func (m MessageID) String() string
func (m MessageID) Valid() bool
```

**Usage**: Used as A2A `message_id` field for correlation and as key for in-flight message tracking.

---

### 4. ChannelID (Value Object)

**Purpose**: Identifies a communication channel (Telegram chat).

**Location**: `internal/domains/gateway/components/ids/channel_id.go`

**Structure**:
```
<provider>:<provider_channel_id>
Example: telegram:123456789
```

**Attributes**:
- `provider` (string): Platform name ("telegram")
- `channelID` (string): Platform-specific channel identifier

**Invariants**:
- Provider must be non-empty (currently only "telegram" supported)
- Channel ID must be non-empty
- Must be parseable from string representation

**Operations**:
```go
func NewChannelID(provider, channelID string) (ChannelID, error)
func (c ChannelID) ProviderID() string
func (c ChannelID) ChannelID() string
func (c ChannelID) String() string
func (c ChannelID) Valid() bool
```

**Usage**: Used as A2A `context_id` for conversation continuity. Maps to Telegram chat ID.

---

### 5. UserID (Value Object)

**Purpose**: Identifies a user across the platform.

**Location**: `internal/domains/gateway/components/ids/user_id.go`

**Structure**:
```
<provider>:<provider_user_id>
Example: telegram:987654321
```

**Attributes**:
- `provider` (string): Platform name ("telegram")
- `userID` (string): Platform-specific user identifier

**Invariants**:
- Provider must be non-empty
- User ID must be non-empty
- Must be parseable from string representation

**Operations**:
```go
func NewUserID(provider, userID string) (UserID, error)
func (u UserID) ProviderID() string
func (u UserID) UserID() string
func (u UserID) String() string
func (u UserID) Valid() bool
```

**Usage**: Identifies message author. Currently informational only (no authentication in MVP).

---

## Domain Services

### 6. Gateway Usecase (Domain Service)

**Purpose**: Orchestrates message flow from Telegram to A2A and back.

**Location**: `internal/domains/gateway/usecases/usecase.go`

**Dependencies** (via Ports):
- `Messenger`: Sends messages to users (Telegram)
- `Agent`: Communicates with A2A server

**Operations**:
```go
func (u *Usecase) ReceiveNewMessageEvent(ctx context.Context, msg *Message) error
```

**Business Logic**:
1. Validate message has text content
2. Notify user processing started (typing indicator)
3. Forward message to A2A server
4. Stream responses back to user via Messenger
5. Handle errors with user-friendly notifications

**Invariants**:
- Messages processed sequentially per channel (for MVP)
- Errors always result in user notification
- Resources cleaned up on completion (goroutines, channels)

**Error Handling**:
- A2A unavailable → User notification
- Network timeout → User notification
- Invalid response → User notification + log
- Empty message → Silent ignore

---

## Ports (Interfaces)

### 7. Agent Port

**Purpose**: Abstracts A2A protocol communication.

**Location**: `internal/domains/gateway/ports/agent.go`

**Contract**:
```go
type Agent interface {
    SendMessage(ctx context.Context, chat MessageID, text MessageText) (iter.Seq2[MessageText, error], error)
}
```

**Semantics**:
- Returns streaming iterator of response chunks
- Iterator yields (text, nil) for successful chunks
- Iterator yields (empty, error) on failure
- Iterator completes when stream ends (io.EOF)
- Context cancellation stops streaming

**Implementation**: `internal/adapters/a2a/client.go`

---

### 8. Messenger Port

**Purpose**: Abstracts messaging platform (Telegram) communication.

**Location**: `internal/domains/gateway/ports/messenger.go`

**Contract**:
```go
type Messenger interface {
    SendMessage(ctx context.Context, channelID ChannelID, text chan MessageText) error
    NotifyProcessingStarted(ctx context.Context, channelID ChannelID) error
}
```

**Semantics**:
- `SendMessage`: Consumes text channel, sends initial message, updates progressively
- Channel closure signals final update
- `NotifyProcessingStarted`: Sends "typing" indicator
- Respects platform rate limits internally
- Returns error only for unrecoverable failures

**Implementation**: `internal/adapters/telegram/client.go`

---

## Data Flow

### Inbound Flow (Telegram → A2A)

```
1. Telegram Webhook POST
   ↓
2. TgBot Controller (parse protobuf)
   ↓
3. Create Message entity
   ↓
4. Gateway Usecase (validate)
   ↓
5. Agent Port (forward to A2A)
   ↓
6. A2A Adapter (gRPC SendStreamingMessage)
```

### Outbound Flow (A2A → Telegram)

```
1. A2A Server streams responses
   ↓
2. A2A Adapter (parse protobuf, yield chunks)
   ↓
3. Gateway Usecase (accumulate in goroutine)
   ↓
4. Messenger Port (batch updates)
   ↓
5. Telegram Adapter (edit message every 3s)
   ↓
6. Telegram Bot API
```

---

## Invariant Enforcement

### Domain Layer
- **Message**: Validates IDs and content on construction
- **MessageText**: Enforces length limits and UTF-8
- **IDs**: Ensure non-empty, parseable structure

### Application Layer
- **Usecase**: Enforces sequential processing per channel
- **Usecase**: Ensures cleanup (defer cancel, close channels)

### Infrastructure Layer
- **Telegram Adapter**: Enforces rate limits via time.Ticker
- **A2A Adapter**: Properly parses protobuf Part messages

---

## Future Extensions (Out of Scope for MVP)

### Multimedia Support
Add value objects:
- `ImageContent`, `VideoContent`, `AudioContent`
- Extend `Message` entity with content union type

### Persistent Conversation History
Add aggregate:
- `Conversation` aggregate with events: MessageReceived, ResponseSent
- Event sourcing for replay and analytics

### Authentication & Authorization
Add value objects:
- `AuthToken` for OAuth tokens
- `UserPermissions` for access control

### Multi-Chat Support
Add:
- `ChatContext` aggregate to track active conversations
- In-memory or Redis-based state management

---

## Domain Glossary

- **Message**: A single communication from a user to the agent
- **Channel**: A communication channel (Telegram chat) where messages are exchanged
- **Context ID**: A2A protocol identifier for conversation continuity (maps to ChannelID)
- **Message ID**: A2A protocol identifier for individual messages
- **Streaming**: Incremental delivery of agent responses
- **Batching**: Accumulating multiple response chunks before updating user
- **Port**: Interface defining integration contract
- **Adapter**: Concrete implementation of a port

---

## Validation Summary

| Entity/Value Object | Validation Rules | Enforcement Point |
|---------------------|------------------|-------------------|
| **Message** | Valid IDs, has content | Constructor |
| **MessageText** | Non-empty, ≤4096 chars, UTF-8 | Constructor |
| **MessageID** | Valid ChannelID, non-empty seq | Constructor |
| **ChannelID** | Non-empty provider & ID | Constructor |
| **UserID** | Non-empty provider & ID | Constructor |
| **Agent responses** | Contains text parts | Adapter parsing |
| **Telegram updates** | Valid message structure | Controller parsing |

All validations return errors rather than panicking, enabling graceful error handling and user notifications.

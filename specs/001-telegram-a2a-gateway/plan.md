# Implementation Plan: Complete Telegram-to-A2A Gateway with Streaming Support

**Branch**: `001-telegram-a2a-gateway` | **Date**: 2025-11-07 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/001-telegram-a2a-gateway/spec.md`

## Summary

Complete the Telegram bot gateway implementation that receives messages via webhook, proxies them to an A2A (Agent-to-Agent) server using gRPC streaming, and returns responses back to users in Telegram. The gateway must support progressive message updates (~3 second intervals) for streaming responses, proper error handling with user notifications, and maintain conversation context across messages. Critical fixes needed: concurrency bug (incorrect WaitGroup usage), missing streaming implementation in Telegram adapter, incorrect A2A response parsing, and suppressed error handling.

**Technical Approach**: Fix existing hexagonal architecture implementation by correcting goroutine management, implementing batched message editing with time-based throttling, properly parsing A2A protobuf responses, and adding comprehensive error propagation with user-friendly notifications.

## Technical Context

**Language/Version**: Go 1.21+
**Primary Dependencies**:
- `google.golang.org/grpc` (A2A gRPC client)
- `google.golang.org/a2a` (A2A protocol buffers)
- `github.com/go-telegram-bot-api/telegram-bot-api/v5` (Telegram Bot API)
- `github.com/google/wire` (dependency injection)
- `github.com/grpc-ecosystem/grpc-gateway/v2` (webhook HTTP/gRPC bridge)

**Storage**: N/A (stateless gateway, no persistence)
**Testing**: Go standard `testing` package, table-driven tests
**Target Platform**: Linux server (containerized deployment)
**Project Type**: Single backend service (gateway/proxy)
**Performance Goals**:
- Handle 10+ concurrent conversations without degradation
- Message forwarding latency <500ms (excluding A2A processing)
- Zero goroutine leaks during continuous operation

**Constraints**:
- Telegram Bot API rate limits (20 msg/sec per bot, 1 edit per message every few seconds)
- Telegram message length limit: 4096 characters
- Must be 100% compatible with A2A protocol (no custom extensions)
- No external state persistence (in-memory only for current iteration)

**Scale/Scope**:
- Initial target: 10-50 concurrent users
- Codebase: ~15 existing files to modify + 3-5 new files
- Estimated 500-1000 LOC changes


## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### Gate 1: Bounded Context Isolation ✅ PASS

**Status**: PASS - No cross-domain imports detected.

**Analysis**: The gateway domain (`internal/domains/gateway`) is properly isolated from the cynosure domain (`internal/domains/cynosure`). The A2A controller in cynosure domain uses its own chat service, while gateway domain uses its own usecase orchestration. No domain types are shared between contexts.

**Action**: None required.

---

### Gate 2: Layered Architecture Integrity ✅ PASS

**Status**: PASS - Existing architecture follows proper layering.

**Analysis**:
- **Domain layer** (`internal/domains/gateway/*`): Contains entities (Message), components (MessageText, IDs), ports (Agent, Messenger), and usecases
- **Application layer** (`internal/apps/gateway/*`): Wire-based dependency injection and orchestration
- **Presentation layer** (`internal/controllers/tgbot/*`): HTTP/gRPC handlers for Telegram webhooks
- **Infrastructure** (`internal/adapters/*`): Concrete implementations (telegram, a2a clients)

Current violations to fix:
- Usecase contains some orchestration logic that should remain thin
- Error handling suppressed in usecase (line 57) - needs proper propagation

**Action**: Ensure error handling improvements maintain layer separation.

---

### Gate 3: Ports & Adapters Purity ⚠️ NEEDS ATTENTION

**Status**: NEEDS ATTENTION - Adapters have implementation issues but architecture is sound.

**Analysis**:
- Port interfaces properly defined in `internal/domains/gateway/ports/`
- Telegram adapter (`internal/adapters/telegram/`) implements `MessengerFactory` port correctly
- A2A adapter (`internal/adapters/a2a/`) implements `AgentFactory` port correctly

Issues to fix:
- Telegram adapter accumulates entire response before sending (breaks streaming contract)
- A2A adapter uses `.String()` method on protobuf (incorrect parsing)
- No business logic in adapters (good), but implementation bugs exist

**Action**: Fix adapter implementations while maintaining port contract purity.

---

### Gate 4: Aggregate Consistency & Event Sourcing Discipline ✅ N/A

**Status**: N/A - This feature does not introduce new aggregates or event sourcing.

**Analysis**: The gateway is a stateless proxy. Entities like `Message` are value objects representing transient webhook data, not domain aggregates requiring event sourcing. No state mutations require event tracking.

**Action**: None required - architecture appropriate for stateless gateway pattern.

---

### Gate 5: Test-First, Contracts & Observability ⚠️ NEEDS ATTENTION

**Status**: NEEDS ATTENTION - Missing comprehensive tests and observability.

**Analysis**:
- Existing code has minimal test coverage
- No contract tests for port implementations
- Basic observability exists (pp.Println debugging statements)
- No structured logging with context keys

**Action Required for Implementation**:
1. Add contract tests for `Agent` and `Messenger` ports
2. Add unit tests for streaming logic, error handling, concurrency fixes
3. Replace debug prints with structured logging (domain, action, context_id)
4. Add metrics for message throughput, error rates, streaming duration

**Merge Gate**: Tests REQUIRED before merge for:
- Concurrency fixes (WaitGroup usage)
- Streaming message accumulation and batching
- A2A response parsing
- Error handling and user notifications

---

### Summary: Pre-Implementation Status

- **2 PASS**: Bounded context isolation, layered architecture foundation
- **1 N/A**: Event sourcing (not applicable to stateless gateway)
- **2 NEEDS ATTENTION**: Adapter implementations, testing & observability

**Gate Decision**: ✅ PROCEED to Phase 0 with remediation plan for attention items.

**Re-check Required**: After Phase 1 design, verify no business logic leaked into adapters during streaming implementation.




## Project Structure

### Documentation (this feature)

```text
specs/001-telegram-a2a-gateway/
├── plan.md              # This file
├── research.md          # Phase 0: Streaming patterns, error handling, rate limiting research
├── data-model.md        # Phase 1: Domain model documentation
├── quickstart.md        # Phase 1: Development & testing guide
└── contracts/           # Phase 1: Port interface contracts
    ├── agent-port.md
    └── messenger-port.md
```

### Source Code (repository root)

```text
internal/
├── domains/
│   └── gateway/                    # Gateway bounded context
│       ├── components/             # Value objects
│       │   ├── message_text.go     # [MODIFY] Add validation for streaming
│       │   └── ids/
│       │       ├── channel_id.go
│       │       ├── message_id.go
│       │       └── user_id.go
│       ├── entities/
│       │   └── message.go          # Telegram message entity
│       ├── ports/
│       │   ├── agent.go            # [MODIFY] Document streaming contract
│       │   ├── messenger.go        # [MODIFY] Document batching contract
│       │   └── wire.go
│       └── usecases/
│           └── usecase.go          # [FIX] Concurrency, error handling
│
├── adapters/
│   ├── telegram/
│   │   └── client.go               # [MAJOR REWRITE] Implement streaming message updates
│   └── a2a/
│       └── client.go               # [FIX] Parse protobuf responses correctly
│
├── controllers/
│   └── tgbot/
│       ├── controller.go           # [MODIFY] Improve error logging
│       └── logs.go                 # [NEW] Structured logging callbacks
│
└── apps/
    └── gateway/
        ├── app.go
        ├── app_constructor.go
        ├── app_adapters.go
        ├── app_usecases.go
        └── wire.go

cmd/
└── cynosure/
    └── root/
        └── gateway/
            ├── cmd.go              # Gateway entrypoint
            └── env.go              # Configuration

contrib/
└── telegram-proto/                 # Existing Telegram protobuf definitions
    └── pkg/telegram/botapi/v9/

tests/                              # [NEW] Test files
├── contract/
│   ├── agent_contract_test.go      # Contract tests for A2A adapter
│   └── messenger_contract_test.go  # Contract tests for Telegram adapter
├── integration/
│   └── gateway_integration_test.go # End-to-end webhook→A2A→Telegram flow
└── unit/
    ├── usecase_test.go             # Usecase logic tests
    ├── telegram_streaming_test.go  # Streaming batching logic tests
    └── a2a_parsing_test.go         # Protobuf response parsing tests
```

**Structure Decision**:

This feature modifies existing hexagonal architecture within a single Go project. The codebase already follows DDD principles with:

- **Bounded contexts**: `cynosure` (agent runtime) and `gateway` (Telegram proxy) are isolated
- **Hexagonal architecture**: Domain → Ports → Adapters → Controllers pattern
- **Dependency injection**: Wire-based DI in `internal/apps/`

**Changes focus on**:
1. Fixing bugs in existing usecase and adapters (no architectural changes)
2. Enhancing Telegram adapter with streaming capabilities (maintains port contract)
3. Adding comprehensive test coverage (new test/ directory)
4. Improving observability (structured logging in controllers)

No new domains, no cross-context dependencies, no layering violations.




## Complexity Tracking

**Status**: No violations requiring justification.

The implementation maintains the existing hexagonal architecture without introducing additional complexity:

- No new bounded contexts
- No new abstraction patterns beyond existing ports/adapters
- No cross-domain imports
- No custom frameworks or reflection-based adapters
- Changes are bug fixes and feature completion within established patterns

All constitution gates pass or have clear remediation paths within the existing architecture.

---

## Phase 0 Summary: Research Completed ✅

**Output**: `research.md`

**Decisions Made**:
1. **Concurrency**: Use context-based cancellation with proper channel closure
2. **Streaming**: Time-based batching with 3-second ticker
3. **Parsing**: Extract text from A2A Part messages (not `.String()`)
4. **Error Handling**: Categorize errors + user-friendly messages + structured logging
5. **Testing**: Unit + contract + integration test strategy
6. **Observability**: Structured logging with `log/slog`

**Key Findings**:
- Identified 4 critical bugs to fix (concurrency, streaming, parsing, error handling)
- Established patterns for rate limiting and message batching
- Defined comprehensive testing strategy with 3-level test pyramid
- Selected standard Go patterns (no new dependencies needed)

---

## Phase 1 Summary: Design Completed ✅

**Outputs**:
- `data-model.md`: Domain entities, value objects, and ports documented
- `contracts/agent-port.md`: Agent port contract with test requirements
- `contracts/messenger-port.md`: Messenger port contract with streaming semantics
- `quickstart.md`: Development guide with examples and debugging tips

**Design Highlights**:
- **Entities**: Message (transient webhook data)
- **Value Objects**: MessageText, MessageID, ChannelID, UserID with validation
- **Ports**: Agent (A2A communication), Messenger (Telegram communication)
- **Domain Service**: Gateway usecase for orchestration
- **Architecture**: Maintains existing hexagonal/DDD structure

**Port Contracts**:
- Agent: Streaming iterator with proper cleanup and error handling
- Messenger: Time-based batching with rate limit compliance
- Both ports have comprehensive contract tests defined

**Agent Context Updated**: ✅
- Language: Go 1.21+
- Storage: N/A (stateless)
- Updated `.github/copilot-instructions.md`

---

## Phase 2: Implementation Tasks (Next Step)

**Not included in `/speckit.plan` command** - Run `/speckit.tasks` to generate `tasks.md`

**Expected Task Breakdown**:
1. Fix concurrency bugs in usecase
2. Fix A2A response parsing in adapter
3. Implement streaming in Telegram adapter
4. Add error handling and user notifications
5. Add structured logging
6. Write contract tests
7. Write unit tests
8. Write integration tests
9. Manual testing and validation

**Estimated Effort**: 3-5 days (based on LOC estimates and complexity)

---

## Constitution Re-Check (Post-Design)

### Gate 1: Bounded Context Isolation ✅ PASS
No changes - design maintains isolation.

### Gate 2: Layered Architecture Integrity ✅ PASS
Design preserves existing layers:
- Domain: Entities, components, usecases
- Ports: Interface definitions (no implementation)
- Adapters: Implementation details (streaming, parsing)
- Controllers: Presentation layer (HTTP handlers)

No business logic added to adapters or controllers.

### Gate 3: Ports & Adapters Purity ✅ PASS
Port contracts formalized in `contracts/` directory:
- Agent port: Clear semantics for A2A streaming
- Messenger port: Clear semantics for batching and rate limiting
- Both ports remain implementation-agnostic

Adapter changes are purely implementation fixes, not architectural changes.

### Gate 4: Aggregate Consistency & Event Sourcing ✅ N/A
Still not applicable - gateway remains stateless.

### Gate 5: Test-First, Contracts & Observability ✅ IMPROVED
Design now includes:
- Contract test specifications for both ports
- Unit test strategies for each fix
- Structured logging patterns (slog)
- Observability hooks for metrics

All requirements now addressed with concrete plans.

**Final Gate Decision**: ✅ APPROVED for implementation (Phase 2)

---

## Artifacts Summary

| Artifact | Status | Location |
|----------|--------|----------|
| Implementation Plan | ✅ Complete | `specs/001-telegram-a2a-gateway/plan.md` |
| Research Document | ✅ Complete | `specs/001-telegram-a2a-gateway/research.md` |
| Data Model | ✅ Complete | `specs/001-telegram-a2a-gateway/data-model.md` |
| Agent Port Contract | ✅ Complete | `specs/001-telegram-a2a-gateway/contracts/agent-port.md` |
| Messenger Port Contract | ✅ Complete | `specs/001-telegram-a2a-gateway/contracts/messenger-port.md` |
| Quickstart Guide | ✅ Complete | `specs/001-telegram-a2a-gateway/quickstart.md` |
| Agent Context Update | ✅ Complete | `.github/copilot-instructions.md` |
| Tasks Breakdown | ⏳ Pending | Run `/speckit.tasks` to generate |

---

## Ready for Next Phase

**Branch**: `001-telegram-a2a-gateway`
**Next Command**: `/speckit.tasks` to generate implementation tasks

**Implementation Readiness**:
- ✅ All research decisions made
- ✅ Architecture design complete
- ✅ Port contracts defined
- ✅ Test strategy established
- ✅ Development guide available
- ✅ Constitution compliance verified

**Estimated Timeline**:
- Implementation: 3-5 days
- Testing: 1-2 days
- Review & deployment: 1 day
- **Total**: ~1 week

**Merge Criteria**:
- All contract tests passing
- Unit test coverage >80% for changed files
- Manual testing scenarios complete
- Constitution compliance verified
- Code review approved


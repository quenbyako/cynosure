# Tasks: Complete Telegram-to-A2A Gateway with Streaming Support

**Feature Branch**: `001-telegram-a2a-gateway`
**Input**: Design documents from `/specs/001-telegram-a2a-gateway/`
**Generated**: 2025-11-07

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3, US4)
- Include exact file paths in descriptions

**Path Convention**: This project follows hexagonal architecture at repository root:
- Domain: `internal/domains/gateway/`
- Adapters: `internal/adapters/`
- Controllers: `internal/controllers/`
- Tests: `tests/`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization - no new infrastructure needed, existing project structure in place

- [ ] T001 Review current codebase structure in `internal/domains/gateway/`, `internal/adapters/`, `internal/controllers/tgbot/`
- [ ] T002 Verify dependencies in `go.mod`: grpc, telegram-bot-api, wire, protobuf
- [ ] T003 [P] Review existing port definitions in `internal/domains/gateway/ports/agent.go` and `messenger.go`

**Status**: Minimal setup - gateway structure already exists, focusing on bug fixes and feature completion

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core fixes that MUST be complete before implementing user stories

**⚠️ CRITICAL**: These bug fixes block all user story work

### Critical Bug Fixes

- [X] T004 Fix concurrency bug in `internal/domains/gateway/usecases/usecase.go:52` - replace `wg.Go()` with proper goroutine + context handling
- [X] T005 Fix channel closure in `internal/domains/gateway/usecases/usecase.go` - add `defer close(textChan)` after goroutine completes
- [X] T006 Fix A2A response parsing in `internal/adapters/a2a/client.go:89` - replace `.String()` with proper Part.Text extraction
- [X] T007 Add `extractTextFromA2AMessage` helper function in `internal/adapters/a2a/client.go` to parse protobuf Part messages

### Infrastructure Improvements

- [X] T008 [P] Add structured logging interface in `internal/controllers/tgbot/logs.go` with `ProcessMessageStart`, `ProcessMessageSuccess`, `ProcessMessageIssue` methods
- [X] T009 [P] Create error categorization helper in `internal/domains/gateway/usecases/errors.go` with `userFriendlyError()` function

**Checkpoint**: Foundation ready - all critical bugs fixed, user story implementation can now begin

---

## Phase 3: User Story 1 - Basic Message Exchange (Priority: P1) 🎯 MVP

**Goal**: Enable basic message forwarding from Telegram to A2A server with response delivery

**Independent Test**: Send text message to bot, verify response received from A2A agent within 5 seconds

### Implementation for User Story 1

- [ ] T010 [US1] Update `internal/domains/gateway/usecases/usecase.go:ReceiveNewMessageEvent` to use context cancellation pattern with `defer cancel()`
- [ ] T011 [US1] Update goroutine in `internal/domains/gateway/usecases/usecase.go` to properly handle context cancellation with `select` statement
- [ ] T012 [US1] Ensure `NotifyProcessingStarted` is called before forwarding message to A2A in `usecase.go`
- [ ] T013 [US1] Update A2A client iterator in `internal/adapters/a2a/client.go` to properly parse Part.Text fields from protobuf
- [ ] T014 [US1] Add error handling in `internal/adapters/a2a/client.go` iterator for io.EOF and streaming errors
- [ ] T015 [US1] Update `internal/adapters/telegram/client.go:SendMessage` to consume channel until closed (basic version, no streaming yet)
- [ ] T016 [US1] Add validation in `internal/adapters/telegram/client.go` for channelID provider (must be "telegram")
- [ ] T017 [US1] Replace debug prints (`pp.Println`) in `internal/controllers/tgbot/controller.go` with structured logging calls
- [ ] T018 [US1] Add logging callbacks instantiation in `internal/apps/gateway/app_constructor.go` with observability integration

### Contract Tests for User Story 1

- [ ] T019 [P] [US1] Create `tests/contract/agent_contract_test.go` with `TestAgentContract_BasicExchange` test
- [ ] T020 [P] [US1] Add `TestAgentContract_InvalidInput` test in `tests/contract/agent_contract_test.go`
- [ ] T021 [P] [US1] Create `tests/contract/messenger_contract_test.go` with `TestMessengerContract_BasicSend` test
- [ ] T022 [P] [US1] Add `TestMessengerContract_NotifyProcessing` test in `tests/contract/messenger_contract_test.go`

### Unit Tests for User Story 1

- [ ] T023 [P] [US1] Create `tests/unit/usecase_test.go` with `TestUsecase_ReceiveNewMessageEvent_Success` test
- [ ] T024 [P] [US1] Add `TestUsecase_ConcurrencyFix` test in `tests/unit/usecase_test.go` to verify no goroutine leaks
- [ ] T025 [P] [US1] Create `tests/unit/a2a_parsing_test.go` with `TestA2AClient_ParsesProtobufCorrectly` test
- [ ] T026 [P] [US1] Add `TestA2AClient_HandlesEmptyContent` test in `tests/unit/a2a_parsing_test.go`

**Checkpoint**: User Story 1 complete - basic message exchange working with proper error handling

---

## Phase 4: User Story 2 - Streaming Response Updates (Priority: P2)

**Goal**: Implement progressive message updates for streaming A2A responses (~3 second intervals)

**Independent Test**: Trigger long A2A response, verify Telegram message updates progressively rather than all at once

### Implementation for User Story 2

- [ ] T027 [US2] Rewrite `internal/adapters/telegram/client.go:SendMessage` to implement time-based batching with `time.Ticker`
- [ ] T028 [US2] Add initial message send logic in `SendMessage` - send first chunk immediately as new message
- [ ] T029 [US2] Implement streaming update loop in `SendMessage` with 3-second ticker for periodic edits
- [ ] T030 [US2] Add `editMessage` helper function in `internal/adapters/telegram/client.go` to handle message editing
- [ ] T031 [US2] Handle "message is not modified" errors gracefully in `editMessage` (ignore these errors)
- [ ] T032 [US2] Add final update logic when text channel closes in `SendMessage`
- [ ] T033 [US2] Add message length validation in `SendMessage` - truncate at 4090 chars with "...[truncated]" indicator
- [ ] T034 [US2] Implement context cancellation handling in streaming loop with `select` statement

### Contract Tests for User Story 2

- [ ] T035 [P] [US2] Add `TestMessengerContract_Streaming` test in `tests/contract/messenger_contract_test.go` with mock Telegram API
- [ ] T036 [P] [US2] Add `TestMessengerContract_RateLimitingCompliance` test in `tests/contract/messenger_contract_test.go`
- [ ] T037 [P] [US2] Add `TestAgentContract_Streaming` test in `tests/contract/agent_contract_test.go`

### Unit Tests for User Story 2

- [ ] T038 [P] [US2] Create `tests/unit/telegram_streaming_test.go` with `TestTelegramAdapter_StreamingBatching` test
- [ ] T039 [P] [US2] Add `TestTelegramAdapter_MessageTruncation` test in `tests/unit/telegram_streaming_test.go`
- [ ] T040 [P] [US2] Add `TestTelegramAdapter_RateThrottling` test in `tests/unit/telegram_streaming_test.go`

**Checkpoint**: User Story 2 complete - streaming responses update progressively in Telegram

---

## Phase 5: User Story 3 - Error Recovery and User Notification (Priority: P2)

**Goal**: Graceful error handling with user-friendly notifications for all failure scenarios

**Independent Test**: Simulate A2A server unavailable, verify user receives clear error message within 10 seconds

### Implementation for User Story 3

- [ ] T041 [US3] Implement `userFriendlyError` function in `internal/domains/gateway/usecases/errors.go` with error categorization
- [ ] T042 [US3] Add error case for `context.DeadlineExceeded` returning "⏱ The agent is taking too long to respond..."
- [ ] T043 [US3] Add error case for `codes.Unavailable` returning "🔌 The agent service is temporarily unavailable..."
- [ ] T044 [US3] Add default error case returning "❌ An unexpected error occurred..."
- [ ] T045 [US3] Update `internal/domains/gateway/usecases/usecase.go` to call `userFriendlyError` on A2A errors
- [ ] T046 [US3] Send error notifications to users via `client.SendMessage` with friendly error text
- [ ] T047 [US3] Update goroutine error handling in `usecase.go` to send errors to user instead of logging silently
- [ ] T048 [US3] Add error logging in `internal/controllers/tgbot/controller.go` using `ProcessMessageIssue` callback
- [ ] T049 [US3] Handle A2A unavailable errors gracefully in `internal/adapters/a2a/client.go` with status code checking

### Contract Tests for User Story 3

- [ ] T050 [P] [US3] Add `TestAgentContract_A2AUnavailable` test in `tests/contract/agent_contract_test.go`
- [ ] T051 [P] [US3] Add `TestMessengerContract_ErrorMessageSend` test in `tests/contract/messenger_contract_test.go`
- [ ] T052 [P] [US3] Add `TestAgentContract_ContextCancellation` test in `tests/contract/agent_contract_test.go`

### Unit Tests for User Story 3

- [ ] T053 [P] [US3] Add `TestUsecase_ErrorHandling_A2AUnavailable` test in `tests/unit/usecase_test.go`
- [ ] T054 [P] [US3] Add `TestUsecase_ErrorHandling_Timeout` test in `tests/unit/usecase_test.go`
- [ ] T055 [P] [US3] Add `TestUserFriendlyError_Categorization` test in `tests/unit/errors_test.go`

**Checkpoint**: User Story 3 complete - all errors result in clear user notifications

---

## Phase 6: User Story 4 - Context Preservation Across Messages (Priority: P3)

**Goal**: Enable multi-turn conversations with context maintained via Telegram chat ID as A2A context_id

**Independent Test**: Send sequence of related messages, verify agent responses demonstrate context awareness

### Implementation for User Story 4

- [ ] T056 [US4] Verify `internal/adapters/a2a/client.go` uses `chat.ChannelID().String()` as `context_id` in A2A request
- [ ] T057 [US4] Verify `internal/adapters/a2a/client.go` uses `chat.String()` as `message_id` in A2A request
- [ ] T058 [US4] Add validation in `internal/adapters/a2a/client.go` to ensure context_id consistency per channel
- [ ] T059 [US4] Document context preservation behavior in code comments

### Contract Tests for User Story 4

- [ ] T060 [P] [US4] Add `TestAgentContract_ContextPreservation` test in `tests/contract/agent_contract_test.go` verifying context_id usage
- [ ] T061 [P] [US4] Add `TestAgentContract_MultipleChannels` test verifying different channels have different contexts

### Unit Tests for User Story 4

- [ ] T062 [P] [US4] Add `TestA2AClient_ContextMapping` test in `tests/unit/a2a_client_test.go`
- [ ] T063 [P] [US4] Add `TestUsecase_ContextIsolation` test in `tests/unit/usecase_test.go`

**Checkpoint**: User Story 4 complete - conversations maintain context across messages

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories and final validation

### Code Quality & Documentation

- [ ] T064 [P] Remove all `pp.Println` debug statements replaced with structured logging
- [ ] T065 [P] Add godoc comments to all public functions in `internal/domains/gateway/`
- [ ] T066 [P] Update README.md with gateway setup instructions (if exists)
- [ ] T067 [P] Add inline code comments explaining streaming logic in `telegram/client.go`
- [ ] T068 [P] Add inline code comments explaining concurrency patterns in `usecase.go`

### Integration Testing

- [ ] T069 Create `tests/integration/gateway_integration_test.go` with full webhook→A2A→Telegram flow test
- [ ] T070 Add concurrent users test in `tests/integration/gateway_integration_test.go` (10+ simultaneous messages)
- [ ] T071 Add long-running stability test in `tests/integration/gateway_integration_test.go` (24 hour simulation)

### Performance & Observability

- [ ] T072 [P] Add metrics for message throughput in `internal/controllers/tgbot/controller.go` (if metrics framework exists)
- [ ] T073 [P] Add metrics for streaming duration in `internal/adapters/telegram/client.go`
- [ ] T074 [P] Add metrics for error rates by category in `internal/domains/gateway/usecases/usecase.go`
- [ ] T075 Verify zero goroutine leaks with pprof during manual testing

### Manual Testing & Validation

- [ ] T076 Run all scenarios from `specs/001-telegram-a2a-gateway/quickstart.md` manually
- [ ] T077 Test Scenario 1: Basic message exchange (send "Hello", verify response)
- [ ] T078 Test Scenario 2: Long streaming response (verify progressive updates every ~3 seconds)
- [ ] T079 Test Scenario 3: A2A server unavailable (verify user-friendly error)
- [ ] T080 Test Scenario 4: Very long response >4096 chars (verify truncation)
- [ ] T081 Test Scenario 5: Concurrent users (5+ users simultaneously)

### Final Checks

- [ ] T082 Run full test suite: `go test ./internal/... ./tests/... -v`
- [ ] T083 Verify test coverage >80% for changed files: `go test -coverprofile=coverage.out`
- [ ] T084 Run linter: `golangci-lint run`
- [ ] T085 Verify constitution compliance against `.specify/memory/constitution.md`
- [ ] T086 Update `.github/copilot-instructions.md` if needed (already done in planning)

---

## Dependencies & Execution Order

### Phase Dependencies

```
Phase 1 (Setup)
    ↓
Phase 2 (Foundational - BLOCKING) ← Must complete before ANY user story
    ↓
    ├─→ Phase 3 (US1 - P1) 🎯 MVP
    ├─→ Phase 4 (US2 - P2)
    ├─→ Phase 5 (US3 - P2)
    └─→ Phase 6 (US4 - P3)
    ↓
Phase 7 (Polish)
```

### User Story Dependencies

- **US1 (Basic Message Exchange)**: No dependencies on other stories - can start immediately after Phase 2
- **US2 (Streaming Updates)**: Builds on US1 but independently testable - enhances message delivery
- **US3 (Error Handling)**: Independent of US1/US2 - adds error recovery across all flows
- **US4 (Context Preservation)**: Independent of others - verifies existing context_id usage

**Key Insight**: All user stories are designed to be independently testable. US2 enhances US1, but US1 works without US2. This enables true incremental delivery.

### Within Each User Story

1. **Implementation tasks first** (T0XX without [P] markers may have internal dependencies)
2. **Contract tests** (all marked [P]) can run in parallel
3. **Unit tests** (all marked [P]) can run in parallel
4. Tests SHOULD be written alongside implementation (not strictly TDD, but close)

### Parallel Opportunities

**Phase 2 (Foundational):**
- T008 (logging interface) || T009 (error helpers) - can run in parallel

**Phase 3 (US1):**
- All contract tests (T019-T022) can run in parallel
- All unit tests (T023-T026) can run in parallel

**Phase 4 (US2):**
- All contract tests (T035-T037) can run in parallel
- All unit tests (T038-T040) can run in parallel

**Phase 5 (US3):**
- All contract tests (T050-T052) can run in parallel
- All unit tests (T053-T055) can run in parallel

**Phase 6 (US4):**
- All contract tests (T060-T061) can run in parallel
- All unit tests (T062-T063) can run in parallel

**Phase 7 (Polish):**
- All documentation tasks (T064-T068) can run in parallel
- All metrics tasks (T072-T074) can run in parallel
- Manual test scenarios (T077-T081) can run in parallel

**Cross-Phase Parallelization** (with sufficient team):
- After Phase 2 completes, ALL user stories (US1, US2, US3, US4) can be worked on in parallel by different developers
- Each story is independently implementable and testable

---

## Parallel Example: Phase 2 (Foundational)

Sequential critical path:
```bash
# Terminal 1: Fix concurrency bugs (MUST be sequential)
task T004  # Fix wg.Go() bug
task T005  # Add channel closure
task T006  # Fix A2A parsing
task T007  # Add parsing helper

# Terminal 2: While above is happening (parallel infrastructure)
task T008  # Add logging interface
task T009  # Add error helpers
```

---

## Parallel Example: User Story 1 (MVP)

Implementation phase:
```bash
# Terminal 1: Usecase fixes
task T010  # Update usecase with context pattern
task T011  # Fix goroutine
task T012  # Ensure NotifyProcessingStarted

# Terminal 2: A2A adapter fixes (parallel, different file)
task T013  # Fix protobuf parsing
task T014  # Add error handling

# Terminal 3: Telegram adapter (parallel, different file)
task T015  # Update SendMessage basic version
task T016  # Add validation

# Terminal 4: Controller improvements (parallel, different file)
task T017  # Replace debug prints
task T018  # Add logging callbacks
```

Testing phase (all parallel):
```bash
# All tests can run in parallel - different test files
task T019 & task T020 &  # Agent contract tests
task T021 & task T022 &  # Messenger contract tests
task T023 & task T024 &  # Usecase unit tests
task T025 & task T026    # A2A parsing unit tests
wait
```

---

## Implementation Strategy

### MVP Scope (Week 1)

**Minimum Viable Product** = Phase 2 (Foundational) + Phase 3 (US1)

- Fixes all critical bugs (concurrency, parsing, error handling foundation)
- Delivers basic message exchange functionality
- Includes contract and unit tests for quality assurance
- Results in working Telegram bot that forwards messages to A2A and returns responses

**Value**: Users can interact with A2A agents via Telegram (core functionality)

**Test**: Send "Hello" to bot, receive agent response - DONE ✓

### Incremental Delivery (Weeks 2-3)

- **US2 (Streaming)**: Enhances UX with progressive updates - independently deliverable
- **US3 (Error Handling)**: Improves reliability with user notifications - independently deliverable
- **US4 (Context)**: Enables conversations - independently deliverable

Each story can be deployed independently after completion, providing incremental value.

### Full Feature (Week 3-4)

- All user stories complete (US1-US4)
- Polish phase for production readiness
- Integration tests and manual validation
- Performance verification and documentation

---

## Task Summary

**Total Tasks**: 86
- Phase 1 (Setup): 3 tasks
- Phase 2 (Foundational): 6 tasks (BLOCKING)
- Phase 3 (US1 - MVP): 17 tasks
- Phase 4 (US2): 14 tasks
- Phase 5 (US3): 15 tasks
- Phase 6 (US4): 8 tasks
- Phase 7 (Polish): 23 tasks

**Test Tasks**: 34 (40% of total)
- Contract tests: 14 tasks
- Unit tests: 14 tasks
- Integration tests: 3 tasks
- Manual tests: 5 scenarios

**Parallel Opportunities**: 52 tasks marked [P] (60% of total)

**Independent Test Criteria**:
- ✅ US1: Send message, receive response (5 seconds)
- ✅ US2: Long response updates progressively (~3 second intervals)
- ✅ US3: A2A down, user sees friendly error (10 seconds)
- ✅ US4: Multiple messages maintain conversation context

**MVP Timeline**: 3-5 days (Phase 2 + Phase 3)
**Full Feature Timeline**: 1-2 weeks (all phases)

---

## Format Validation ✅

All tasks follow required format:
- ✅ Checkbox: All tasks start with `- [ ]`
- ✅ Task ID: All tasks have sequential IDs (T001-T086)
- ✅ [P] markers: 52 tasks marked as parallelizable
- ✅ [Story] labels: All user story tasks labeled (US1, US2, US3, US4)
- ✅ File paths: All implementation tasks include specific file paths
- ✅ Descriptions: All tasks have clear, actionable descriptions

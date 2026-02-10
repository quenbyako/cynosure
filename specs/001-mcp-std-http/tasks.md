# Implementation Tasks: Dual-Protocol MCP Support with OAuth (001-mcp-std-http)

*Generated from Phase 1 design. Tasks ordered by dependency.*

**Total Estimated Effort**: 28-36 hours (with TASK-0)
**Critical Path**: P1 → P2 → TASK-0 → TASK-2 → TASK-3 → TASK-7
**Phased Approach**: Phase 1.0 (Week 1) = P1, P2, TASK-0 through TASK-9

## Prerequisites (Must Complete First)

### TASK-P1: Fix Cache Thread-Safety Bug ⚠️ CRITICAL

**File**: `contrib/sf-cache/cache.go`

**Current State**: Known TODO comment `// VERY VERY IMPORTANT: method is not thread-safe`

**Context**: With protocol fallback + OAuth injection, concurrent access patterns increase dramatically. At 2000 clients, race conditions WILL occur.

**Resource Impact Analysis** (from research.md):

- Cache `maxSize` is CONFIGURABLE (not hard-coded to 5)
- LRU eviction algorithm: when at capacity, **gracefully close** (via `Close()`) the connection with oldest activity (request/response/server ping)
- "Last activity" timestamp updated on: ExecuteTool request, response received, server-side SSE ping
- Under limit → add new connection; at limit → close LRU connection, then add new
- **Risk**: Cache contention causes data corruption, not just performance

**Acceptance Criteria**:

- [X] Remove unsafe concurrent access from LRU cache
- [X] Protect `m` map access with `sync.RWMutex` or `sync.Map`
- [X] LRU eviction calls `destructor(client)` which must invoke `client.Close()`
- [X] "Last activity" timestamp tracked per connection (read/write operations)
- [X] All cache operations (Get, Set, Delete, Clear) are protected
- [X] Concurrent access tests verify no data races
- [X] Load test with 500+ concurrent clients passes
- [X] Verify LRU connection properly closed when maxSize reached

**Description**:
Replace unsafe LRU cache operations with proper locking. Use `sync.RWMutex` for read-heavy workload (most operations are Get). Ensure LRU eviction gracefully closes connections via destructor callback. Add comprehensive concurrent tests.

**Estimated Effort**: 2-3 hours

**Acceptance Evidence**:

- `go test -race ./contrib/sf-cache/...` passes without warnings
- Load test: 500 concurrent ExecuteTool calls complete successfully
- Benchmark: p95 latency <10ms for cache operations

**Blocking**: ALL implementation tasks (TASK-0, TASK-1, TASK-2, TASK-3)

---

### TASK-P2: Fix Context Propagation in OAuth Refresher

**File**: `internal/adapters/oauth/oauth_refresher.go`

**Current State**: Uses `context.TODO()` for token refresh context (violates timeout propagation)

**Impact**: Token refresh can hang indefinitely if request context cancelled

**Changes Required**:

1. Locate `newRefresher()` call with `context.TODO()`
2. Replace with `context.WithoutCancel(requestCtx)`
3. Add explanatory comment: `// WithoutCancel preserves deadline but detaches from request cancellation`
4. Rationale: Token refresh must complete even if user cancels request, but MUST respect timeout

**Acceptance Criteria**:

- [X] No `context.TODO()` remains in oauth_refresher flow
- [X] Token refresh uses `context.WithoutCancel(ctx)` for background refresh
- [X] Test: Request cancelled → token refresh still completes
- [X] Test: Timeout reached → token refresh fails appropriately

**Estimated Effort**: 1 hour

**Acceptance Evidence**:

- Code review confirms proper context usage
- Unit test demonstrates timeout respected but cancellation ignored

**Blocking**: TASK-0 (Bearer token injection needs correct context)

---

## Phase 2a: OAuth Foundation (CRITICAL - Before Protocol Fallback)

### TASK-0: Implement Bearer Token Injection Wrapper 🔐 NEW & CRITICAL

**File**: `internal/adapters/tool-handler/bearer_wrapper.go` (NEW)

**Priority**: BLOCKS ALL fallback logic (TASK-1, TASK-2, TASK-3)

**Problem Statement**:

Current: OAuth tokens obtained but NOT injected → 401 errors from protected servers → Fallback doesn't help

**Description**:

Create HTTP transport wrapper that automatically injects Bearer tokens from existing OAuth infrastructure into ALL MCP requests. Works identically for both Streamable HTTP and legacy SSE transports.

**Acceptance Criteria**:

- [X] **TDD**: Write unit tests BEFORE implementation (token injection, missing token, 401 response)
- [X] `BearerTokenTransport` implements `http.RoundTripper` interface
- [X] Authorization header format: `Authorization: Bearer <token>` (RFC 7235)
- [X] Integrates with existing `oauth/handler.go` token storage
- [X] Works identically for Streamable HTTP and SSE transports
- [X] Handles missing/expired tokens gracefully
- [X] Unit tests achieve >90% coverage

**Estimated Effort**: 3-4 hours

**Acceptance Evidence**:

- Unit tests pass with >90% coverage
- Integration test: Connect to mock protected server (both protocols)
- Bearer token visible in captured HTTP requests

**Dependencies**: TASK-P2 (context propagation)

**Blocking**: TASK-1, TASK-2, TASK-3 (all protocol fallback logic)

---

## Phase 2b: Core Implementation

### TASK-1: Define Transport Error Types

**File**: `internal/adapters/tool-handler/errors.go` (NEW)

**Description**:
Create typed error system to distinguish between infrastructure errors (fail immediately) and protocol errors (trigger fallback).

**Acceptance Criteria**:
- [X] **TDD**: Write unit tests for `classifyError` BEFORE implementation (covers: Infrastructure, Protocol, Auth errors)
- [X] `TransportError` interface defines error classification methods
- [X] `InfrastructureError` type (connection refused, DNS, TLS rejection)
- [X] `ProtocolError` type (malformed response, unexpected EOF, unknown response)
- [X] `AuthError` type (401, 403, token expired)
- [X] `classifyError(error)` function maps SDK/network errors to typed errors
- [X] `isProtocolError(error)` returns true only for logical protocol mismatches
- [X] Unit tests cover error classification for representative cases

**Key Decisions**:
- Protocol errors (A1): unknown response, unexpected EOF → fallback
- Infrastructure errors (A1): connection refused, DNS, TLS → fail immediately
- Error synthesis (B2): "address is not an MCP server (both protocols failed)"

**Estimated Effort**: 1-2 hours

**Acceptance Evidence**:
- All error types defined
- Unit tests for classifyError with 5+ test cases
- Code review confirms error semantics

**Dependencies**: None

---

### TASK-2: Implement Fallback Logic in Handler

**File**: `internal/adapters/tool-handler/handler.go`

**Function**: `newAsyncClient(ctx, url, httpClient)`

**Description**:
Implement protocol fallback: attempt Streamable first, fall back to SSE only on protocol errors. Infrastructure errors fail immediately.

**Acceptance Criteria**:
- [X] **TDD**: Write fallback scenario tests BEFORE implementation
- [X] Try StreamableClientTransport first with MaxRetries=0 (per A3)
- [X] Classify error returned from Streamable attempt
- [X] Only fall back on ProtocolError (not InfrastructureError or AuthError)
- [X] If Streamable succeeds, use it (no SSE attempt)
- [X] If Streamable fails with protocol error, try SSEClientTransport
- [X] If SSE also fails, return classified error (per B2: "address is not an MCP server...")
- [X] Unified timeout applied to both transports (per C1)
- [X] Unit tests for success, fallback, and fail-fast scenarios

**Implementation Pattern** (per architectural invariants):
```go
// Attempt Streamable (modern protocol)
session, err := h.connectWithTransport(ctx,
    &StreamableClientTransport{...},
    url, httpClient)

// Classify error
if err != nil && h.isProtocolError(err) {
    // Only protocol errors trigger fallback
    session, err = h.connectWithTransport(ctx,
        &SSEClientTransport{...},
        url, httpClient)
}

// If both fail, return synthesized error or classified error
if err != nil {
    return nil, wrapTransportError(err)
}

return &asyncClient{session: session, ...}, nil
```

**Key Decisions**:
- Fallback on protocol errors only (A1)
- MaxRetries=0 for protocol mismatches (A3: no retry logic needed)
- Same timeout for both (C1)
- Synthetic error message for "not an MCP server" (B2)
- Don't log protocol errors (B1)

**Estimated Effort**: 3-4 hours

**Acceptance Evidence**:
- Unit tests pass for: successful Streamable, fallback to SSE, fail-fast on infrastructure error
- Code review confirms proper error classification
- Concurrent load test with 100+ simultaneous clients

**Dependencies**: TASK-P1, TASK-P2, TASK-1

---

### TASK-3: Implement Protocol Detection in RegisterTools

**File**: `internal/adapters/tool-handler/registry.go`

**Function**: `RegisterTools(ctx, serverInfo, httpClient)`

**Description**:
Probe server capabilities during tool registration. Detect which protocol(s) the server supports and record for future use (groundwork for A2 persistence).

**Acceptance Criteria**:
- [X] During tool registration, probe both Streamable and SSE
- [X] Determine which protocol server supports first
- [X] Record supported protocols via `server.UpdateSupportedProtocols()` method
- [X] Handle case where only one protocol succeeds
- [X] Handle case where both fail (return "not an MCP server" error)
- [X] Store result for future protocol preference (A2 groundwork)
- [X] Unit tests for protocol detection scenarios

**Key Decisions**:
- Protocol detection happens at registration time (D1)
- Enables future persistence of protocol preference (A2)
- Sets up observability hooks for adoption metrics (Future phase)

**Estimated Effort**: 2-3 hours

**Acceptance Evidence**:
- Protocol detection works in RegisterTools
- SupportedProtocols field populated correctly
- Test scenarios: Streamable-only, SSE-only, both, neither

**Dependencies**: TASK-1, TASK-2

---

### TASK-4: Extend Server Entity with Protocol Fields

**File**: `internal/domains/cynosure/entities/server.go` (or similar)

**Description**:
Add protocol awareness fields to Server aggregate. This is groundwork for future persistence (A2); don't implement persistence yet.

**Acceptance Criteria**:
- [X] `Server` struct has private protocol fields
- [X] `Server` struct has `UpdateSupportedProtocols([]string)` method to encapsulate changes
- [X] `Server` struct has `PreferredProtocol *string` field (optional)
- [X] Fields are properly tagged for storage (if using ORM)
- [X] No database migration yet (just entity structure)
- [X] Domain model updated consistently

**Key Decisions**:
- Add fields but don't persist (A2 groundwork)
- PreferredProtocol is optional (future feature)
- SupportedProtocols is ordered (first element is preferred)

**Estimated Effort**: 1 hour

**Acceptance Evidence**:
- Entity compiles
- Fields are readable/writable
- Code review confirms DDD alignment

**Dependencies**: None

---

### TASK-5: Implement ServerStorage Port Update

**File**: `internal/domains/cynosure/ports/` (ServerStorage interface)

**Description**:
Update ServerStorage port to support reading/writing protocol information. This is architectural groundwork; don't implement database logic yet.

**Acceptance Criteria**:
- [X] ServerStorage interface has method to read SupportedProtocols (or retrieve Server with protocols)
- [X] ServerStorage interface supports saving SupportedProtocols (or updating Server with protocols)
- [X] Method signatures are added but SQL implementation deferred
- [X] Adapter (SQL) stubs are created but not implemented

**Key Decisions**:
- Port definition only (A2 groundwork)
- SQL implementation is future task
- No database migration yet

**Estimated Effort**: 1-2 hours

**Acceptance Evidence**:
- Interface methods defined
- Stubs in SQL adapter compile
- Code review confirms port design

**Dependencies**: TASK-4

---

### TASK-6: [MERGED] Unit Tests for Error Classification

**Description**:
*Merged into TASK-1 and TASK-2 to comply with TDD Constitution constraint.*

See TASK-1 for error classification tests and TASK-2 for fallback logic tests.

---

### TASK-7: Add Integration Tests for Concurrent Fallback

**File**: `internal/adapters/tool-handler/handler_test.go`

**Description**:
Integration tests simulating realistic concurrent scenarios with protocol fallback.

**Test Scenarios**:
- [ ] 100 concurrent clients, Streamable succeeds for all
- [ ] 100 concurrent clients, all Streamable fails → fallback to SSE succeeds
- [ ] Mixed: 50 Streamable, 50 fallback to SSE (concurrent)
- [ ] Cache consistency: same server contacted multiple times uses cached protocol
- [ ] Context cancellation: timeout applied correctly to both transports
- [ ] OAuth refresh during fallback: token refresh doesn't block protocol negotiation

**Acceptance Criteria**:
- [ ] All integration tests pass
- [ ] No data races detected (`go test -race`)
- [ ] Load test with 500+ concurrent clients completes without timeout
- [ ] Cache thread-safety verified under concurrency

**Estimated Effort**: 3-4 hours

**Acceptance Evidence**:
- `go test -race ./internal/adapters/tool-handler` passes
- Load test report shows all 500+ clients succeeded
- No deadlocks or race condition warnings

**Dependencies**: TASK-P1, TASK-2, TASK-3

---

## Validation Tasks

### TASK-8: Code Review & Architecture Validation

**Description**:
Ensure implementation aligns with architectural principles and DDD boundaries.

**Checklist**:
- [ ] Error types don't leak transport details to domain layer
- [ ] Adapter layer is responsible for protocol negotiation (not domain)
- [ ] ServerStorage port changes don't violate bounded context
- [ ] Concurrency safety is verified (no TODO comments remain)
- [ ] Context propagation is correct throughout fallback flow
- [ ] Performance: protocol detection doesn't add significant latency
- [ ] Error messages are actionable for operators

**Estimated Effort**: 1-2 hours

**Acceptance Evidence**:
- Code review sign-off
- Architecture diagram updated (optional)
- Performance baseline documented

**Dependencies**: All TASK-1 through TASK-7

---

### TASK-9: Documentation & Runbook

**Description**:
Update internal documentation for operators and future developers.

**Artifacts**:
- [X] README update: Dual-protocol support explanation
- [X] Error handling guide: How to interpret fallback errors
- [X] Runbook: Debugging protocol mismatch issues
- [X] Comments in code for non-obvious fallback logic
- [ ] Architecture diagram: Transport selection flow (textual diagram provided)

**Estimated Effort**: 2 hours

**Acceptance Evidence**:
- Documentation is clear to someone unfamiliar with code
- Runbook covers common scenarios (all Streamable, all SSE, mixed)

**Dependencies**: All implementation tasks

---

## Summary Statistics

| Metric | Value |
| --- | --- |
| **Total Tasks** | 8 (2 prerequisites + 6 implementation) |
| **Total Estimated Effort** | 20-30 hours |
| **Longest Path** | TASK-P1 → TASK-P2 → TASK-2 → TASK-3 → TASK-7 |
| **Critical Path Duration** | ~13 hours (sequential critical dependencies) |
| **Parallelizable Work** | TASK-1, TASK-4 can run in parallel with TASK-2 |
| **Testing Effort** | TASK-7 = 3-4 hours (Unit tests included in dev tasks) |

## Execution Strategy

**Phase 2a: Blockers** (4-5 hours)
1. TASK-P1: Cache thread-safety fix
2. TASK-P2: OAuth context fix

**Phase 2b: Core Implementation** (9-12 hours, parallelizable)
1. TASK-1: Error types (parallel with TASK-4)
2. TASK-4: Server entity fields
3. TASK-2: Fallback logic (depends on TASK-1)
4. TASK-3: Protocol detection (depends on TASK-2)

**Phase 2c: Testing** (3-4 hours)
1. TASK-7: Integration tests (Unit tests completed in Phase 2b)

**Phase 2d: Polish** (3-4 hours)
1. TASK-5: ServerStorage port update (can start earlier)
2. TASK-8: Code review
3. TASK-9: Documentation

## Risk Mitigation

**Risk**: Cache thread-safety not fixed before concurrent tests
**Mitigation**: TASK-P1 is hard prerequisite; verify with `go test -race` before TASK-7

**Risk**: Context propagation issues cause token refresh failures
**Mitigation**: TASK-P2 fix + specific unit test for refresh during fallback

**Risk**: Protocol detection changes affect tool listing performance
**Mitigation**: Add latency benchmark to TASK-7; limit protocol probes to registration only (not per-execution)

**Risk**: Error classification misses edge cases in production
**Mitigation**: Add comprehensive error classification unit tests (TASK-6); consider adding error metric logging in Phase 3

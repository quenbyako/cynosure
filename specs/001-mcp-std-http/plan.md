# Implementation Plan: Dual-Protocol MCP Support

**Branch**: `001-mcp-std-http` | **Date**: 2026-01-24 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/001-mcp-std-http/spec.md`

**Status**: Ready for implementation | Phase 0 & 1 Complete

## Summary

Implement dual-protocol connection strategy for MCP clients with OAuth Bearer token support. The adapter attempts **Streamable HTTP** (MCP 2025-11-25) first, then falls back to **legacy HTTP+SSE** (MCP 2024-11-05) only on protocol compatibility errors (HTTP 400/404/405). Infrastructure errors (DNS, TLS, connection refused) fail immediately without fallback. OAuth Bearer tokens are injected consistently across both protocols.

**Key Principles**:

- **Streamable HTTP = First-Class Citizen** (modern, prioritized)
- **Legacy SSE = Degraded Mode** (backward compatibility)
- **Bearer Token Injection = Protocol-Independent** (same auth for both)
- **Fallback = Logical** (protocol selection, NOT retry)

**Critical Gap Addressed**: Current code obtains OAuth tokens but doesn't inject them into MCP requests → TASK-0 adds Bearer token wrapper before protocol fallback logic.

## Technical Context

**Language/Version**: Go 1.25.1

**Primary Dependencies**:

- `github.com/modelcontextprotocol/go-sdk` v1.2.0
  - `StreamableClientTransport`: Modern protocol (2025-11-25)
  - `SSEClientTransport`: Legacy protocol (2024-11-05)
  - `oauthex` package: OAuth extensions (resource discovery, auth server metadata)
  - `auth` package: Token verification (server-side)

**Storage**: Server entity extended with protocol awareness (groundwork only, no persistence yet)

**Testing**: Go standard testing + concurrent client simulation (2000+ clients)

**Target Platform**: Linux server / Cloud

**Project Type**: Backend Service

**SDK Capabilities**:

- ✅ Both transports available and tested
- ✅ Same URL works for both protocols
- ✅ OAuth metadata discovery (RFC 9728, RFC 8414)
- ✅ PKCE utilities available
- ❌ Bearer token injection NOT provided (we must build)
- ❌ Client-side OAuth 2.1 flow NOT complete (deferred to Phase 1.1)

**Current OAuth Infrastructure**:

- ✅ `oauth/handler.go`: Metadata discovery, token exchange
- ✅ `oauth_refresher.go`: Token refresh mechanism
- ❌ **CRITICAL GAP**: Tokens obtained but NOT injected into MCP requests
- ❌ **BUG**: Uses `context.TODO()` instead of proper context propagation

**Constraints**:

- Protocol-aware fallback (NOT infrastructure retry)
- Fix cache thread-safety bug as **mandatory** prerequisite
- Fix context propagation in oauth_refresher
- Unified timeouts for both transports (no per-transport override)
- Bearer token injection must work identically for both protocols

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Bounded Context Isolation**: PASS. Change is localized to `internal/adapters/tool-handler`.
- **Layered Architecture Integrity**: PASS. Logic resides in adapter/infrastructure layer; domain layer stays unaware.
- **Ports & Adapters Purity**: PASS. Implements `ports.ToolManagerFactory`; custom error types classify transport errors.
- **Aggregate Consistency**: N/A (no domain state mutations).
- **Test-First**: PASS. Will write tests for error classification and protocol detection.
- **Concurrency Safety**: MUST FIX. Cache thread-safety bug is blocker for 2000+ concurrent clients.
- **Observability**: Deferred to Phase 2 (Prometheus metrics for adoption tracking).

## Implementation Options & Decision

### Option A: Full Spec-Compliant (Production-Ready Day 1)

**Timeline**: 3 weeks | **Effort**: 35-46 hours

**Scope**:

- ✅ Dual-protocol fallback (Streamable → Legacy SSE)
- ✅ Bearer token injection
- ✅ Full OAuth 2.1 client flows (PKCE, resource indicators RFC 8707)
- ✅ Protected resource discovery (RFC 9728)
- ✅ Scope management + step-up authorization
- ✅ MCP Authorization Spec 2025-11-25 compliant

**Pros**: Works with ANY MCP server (internal + external), secure, compliant

**Cons**: Higher complexity, longer timeline, more testing needed

**Week Breakdown**:

- Week 1: Prerequisites (P1, P2) + Bearer wrapper (TASK-0) + Error types (TASK-1)
- Week 2: Fallback logic (TASK-2-3) + Full OAuth flows (X1-X4)
- Week 3: Integration testing + Review + Documentation

---

### Option B: Phased Approach ⭐ RECOMMENDED

**Phase 1.0 Timeline**: 1 week | **Effort**: 18-24 hours
**Phase 1.1 Timeline**: +1 week | **Effort**: +12-16 hours

**Phase 1.0 Scope** (Week 1):

- ✅ Dual-protocol fallback (Streamable → Legacy SSE)
- ✅ Bearer token injection for pre-configured OAuth tokens
- ✅ Works with internal MCP servers that have static auth
- ✅ Backward compatibility (2024-11-05 + 2025-11-25)

**Phase 1.1 Scope** (Week 2, separate feature):

- ✅ Full OAuth 2.1 client flows (PKCE)
- ✅ Dynamic resource discovery
- ✅ Scope management

**Pros**: Fast visible result (Week 1), iterative feedback, lower risk, separates concerns

**Cons**: Need phased rollout, some refactoring between phases

**Rationale**: Dual-protocol and OAuth are **orthogonal concerns**—both protocols use same tokens, same auth server. Implement protocol fallback first (core feature), then enhance with dynamic OAuth flows (security hardening).

---

### Decision: Option B Selected ✅

**Reasoning**:

1. **Faster time to value**: Dual-protocol works in 1 week
2. **Lower risk**: Smaller changes per phase, easier to test
3. **Clear milestones**: Phase 1.0 = protocol support, Phase 1.1 = OAuth enhancement
4. **OAuth infrastructure exists**: Pre-configured tokens already work, just need injection
5. **Backward compatible**: Can deploy Phase 1.0 immediately for internal servers

**Release Plan**:

- **Release 1.0** (Week 1): Streamable + SSE fallback + Bearer injection → Internal servers ✅
- **Release 1.1** (Week 2+): Add OAuth 2.1 flows → External servers ✅
- **Release 1.2+** (Future): Observability, protocol persistence, advanced features

---

## OAuth Integration Architecture

### Current State Analysis

**Existing OAuth Infrastructure** (`internal/adapters/oauth/`):

```
✅ handler.go:
   - getServerMetadata() attempts RFC 9728 Protected Resource discovery
   - fetchProtectedResource() queries OAuth endpoints
   - fetchMetadataFromURL() retrieves auth server metadata
   - State management with COSE encryption
   - Token exchange with authorization server

✅ oauth_refresher.go:
   - Automatic token refresh mechanism
   - ❌ BUG: Uses context.TODO() → breaks timeout propagation

❌ CRITICAL GAP: NO bearer token injection into MCP HTTP requests!
```

**Problem Flow**:

```
oauth_refresher.Exchange() → ✅ Token obtained
                           → ❌ Token NOT added to request headers

newAsyncClient(Streamable/SSE) → ❌ Client created WITHOUT Authorization header
                                → MCP request sent without auth
                                → 401 Unauthorized from protected servers
```

### Solution: Bearer Token Injection Layer (TASK-0)

**Architecture**:

```go
type BearerTokenTransport struct {
    base http.RoundTripper
    getToken func(ctx context.Context) (string, error)
}

func (b *BearerTokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
    // 1. Retrieve token from OAuth infrastructure
    token, _ := b.getToken(req.Context())

    // 2. Inject into Authorization header (RFC 7235)
    if token != "" {
        req.Header.Set("Authorization", "Bearer "+token)
    }

    // 3. Forward request
    resp, err := b.base.RoundTrip(req)

    // 4. Handle 401 responses (trigger resource discovery)
    if resp != nil && resp.StatusCode == 401 {
        // Extract WWW-Authenticate header, initiate RFC 9728 discovery
        // (Integration with existing oauth/handler.go logic)
    }

    return resp, err
}
```

**Integration Point** (`handler.go`):

```go
func (h *Handler) newAsyncClient(ctx, url, httpClient) {
    // TASK-0: Wrap HTTP client with bearer token injection
    httpClient.Transport = &BearerTokenTransport{
        base: httpClient.Transport,
        getToken: func(ctx context.Context) (string, error) {
            return h.accounts.GetToken(ctx, accountID)
        },
    }

    // Proceed with protocol fallback (TASK-2)
    // Both Streamable AND SSE now have Bearer tokens!
    session, err := h.connectWithFallback(ctx, url, httpClient)
    ...
}
```

**Key Properties**:

- ✅ **Protocol-independent**: Works identically for Streamable HTTP and SSE
- ✅ **Non-intrusive**: Wraps existing HTTP client, no protocol logic changes
- ✅ **Reuses infrastructure**: Integrates with existing oauth/handler.go
- ✅ **Handles auth errors**: 401 triggers resource discovery (RFC 9728)

**Effort**: 3-4 hours (TASK-0, prerequisite for all fallback tasks)

---

## Protocol Version Compatibility

### MCP Protocol Evolution

**Old Protocol** (2024-11-05): HTTP+SSE

```
Architecture: ASYMMETRIC endpoints
  - POST to /sse → send client requests
  - GET from /sse → receive SSE stream with responses

First SSE event: `endpoint` event with metadata
```

**New Protocol** (2025-11-25): Streamable HTTP

```
Architecture: UNIFIED endpoint
  - POST or GET to /mcp → single endpoint handles both
  - Response: Either JSON-RPC directly OR SSE stream

Content negotiation: Accept header determines format
```

### Backward Compatibility Strategy

**Detection Logic**:

```
Step 1: POST InitializeRequest (Streamable 2025-11-25)
        Headers: Accept: application/json, text/event-stream

Step 2a: HTTP 2xx → ✅ New protocol (Streamable)
         Continue with this transport

Step 2b: HTTP 400/404/405 → ⚠️ Protocol mismatch
         Server doesn't understand Streamable
         → Try old protocol

Step 3: GET to same URL (Old HTTP+SSE 2024-11-05)
        Headers: Accept: text/event-stream

Step 4: Receive SSE stream
        First event = `endpoint` → ✅ Old protocol confirmed
        Continue with legacy transport
```

**Error Classification for Fallback**:

```go
// ONLY these HTTP statuses trigger fallback:
// 400 Bad Request   → POST body not understood (old server)
// 404 Not Found     → Endpoint doesn't exist (wrong path)
// 405 Method Not Allowed → POST not supported (old server)

// These do NOT trigger fallback (fail immediately):
// 401 Unauthorized  → Auth error (not protocol)
// 403 Forbidden     → Permissions (not protocol)
// 5xx Server Error  → Server issue (not protocol)
// Network errors    → Infrastructure failure
```

**Protocol Detection & Storage** (TASK-3):

- During `RegisterTools()`, probe which protocol(s) server supports
- Record in `Server.SupportedProtocols []string` (groundwork for persistence)
- First entry = preferred protocol for future connections
- Enables observability: track protocol adoption metrics

---

## Scale & Concurrency Analysis

### Current Bottlenecks

**Cache Configuration**:

```go
cache.New(constructor, destructor, maxSize=configurable, ttl=10*time.Minute)
```

**Purpose**: LRU cache limits active MCP connections with graceful eviction

- `maxSize` is CONFIGURABLE (e.g., 10, 100, 2000) based on deployment capacity
- When limit reached: LRU connection (least recently used) is **gracefully closed** via `Close()`
- "Last activity" = most recent request/response/server-side ping
- Under limit: new connections added immediately
- At limit: oldest idle connection closed before creating new one

**Thread-Safety Issue** (`contrib/sf-cache/cache.go`):

```go
// TODO: VERY VERY IMPORTANT: method is not thread-safe.
```

**Risk**: Race conditions at scale (1000+ concurrent goroutines)

- Cache.Get() has known concurrency bug
- Fallback logic increases concurrent access patterns
- Data corruption or deadlocks possible under load

**MANDATORY Prerequisite** (TASK-P1): Fix cache thread-safety

- Replace unsafe LRU access with `sync.RWMutex`
- Add concurrent access tests (`go test -race`)
- Verify with load test (500+ clients)

### Resource Impact: 2000 Concurrent Clients

| Resource | Streamable HTTP | Legacy SSE | Mitigation |
|----------|----------------|------------|------------|
| **File Descriptors** | 2000 FDs | 2000 FDs | Increase ulimit (65536) |
| **Memory per conn** | ~20-50 KB | ~50-100 KB | Total: ~150 MB (acceptable) |
| **TCP connections** | 2000 | 2000 | HTTP/2 multiplexing helps |
| **Context goroutines** | 1/client | 1/client | Bounded, manageable |

**Conclusion**: 2000 clients **FEASIBLE** if:

1. Cache maxSize configured to deployment capacity (e.g., maxSize=2000)
2. Thread-safety fixed (TASK-P1, this feature)
3. OS limits raised (`ulimit -n 65536`)
4. LRU eviction properly closes idle connections via `Close()`

---

## Project Structure

### Documentation (this feature)

```text
specs/001-mcp-std-http/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

**Files to modify:**

```text
internal/
└── adapters/
    └── tool-handler/
        ├── handler.go           # Modify: newAsyncClient() with fallback
        ├── errors.go            # NEW: Custom error types for protocol classification
        ├── handler_test.go       # NEW/Modify: Fallback logic tests
        └── registry.go           # Modify: RegisterTools to detect protocol

internal/adapters/
└── oauth/
    └── oauth_refresher.go       # Modify: Fix context.TODO()

contrib/
└── sf-cache/
    └── cache.go                 # Modify: Fix thread-safety TODO

internal/domains/cynosure/
└── entities/
    └── server.go                # Modify: Add SupportedProtocols, PreferredProtocol fields
```

**Scope Decision**: Modified existing adapter. No new adapter created.

## Implementation Strategy by Phase

### Phase 0: Resolve Unknowns ✅ COMPLETED
- SDK analysis: Both transports confirmed available
- Decision mapping: All А1-Е2 questions answered with architectural context
- Error classification: Protocol vs infrastructure errors defined

### Phase 1: Design & Contracts

**1.1 Error Type Design**

Custom error types in new `errors.go`:
```go
type TransportError interface {
    error
    IsInfrastructure() bool  // connection refused, DNS, TLS
    IsProtocol() bool        // unknown response, EOF, malformed
    IsAuth() bool            // 401, 403, token expired
}

type InfrastructureError struct { ... }  // fail immediately
type ProtocolError struct { ... }        // trigger fallback
type AuthError struct { ... }            // update token, not fallback
```

**1.2 Protocol Detection**

Modify `RegisterTools`:
1. Attempt Streamable (fast protocol probe)
2. Classify error: infrastructure → fail; protocol → try SSE
3. Record working protocol in SupportedProtocols field (groundwork for А2)

**1.3 Context Propagation**

Fix `oauth_refresher.go`:
```go
newRefresher(
    context.WithoutCancel(ctx),  // ← FIX: proper context propagation
    auth,
    accounts,
    session,
    serverInfo,
)
```

**1.4 Domain Model Extension** (groundwork, no persistence yet)

Modify `Server` entity:
```go
type Server struct {
    // existing fields
    supportedProtocols []string  // private to enforce invariant via method
    preferredProtocol  *string   // optional: future use
}

func (s *Server) UpdateSupportedProtocols(protocols []string) {
    s.supportedProtocols = protocols
    // s.AddDomainEvent(NewServerProtocolsUpdatedEvent(s.ID, protocols))
}
```

**1.5 Thread-Safety Fix**

Fix `cache.go`:
- Replace unsafe LRU access with proper locking
- Use `sync.RWMutex` to protect concurrent reads/writes
- Add concurrent access tests

### Phase 2: Implementation (actual coding)

**2.1 Fallback Logic in `newAsyncClient`**
```go
func newAsyncClient(ctx context.Context, u *url.URL, httpClient *http.Client) (*asyncClient, error) {
    // Try Streamable first
    session, err := connectWithTransport(ctx, &StreamableClientTransport{...})

    // Only fallback on protocol errors
    if err != nil && isProtocolError(err) {
        session, err = connectWithTransport(ctx, &SSEClientTransport{...})
    }

    // Infrastructure/auth errors: fail immediately (no fallback)
    if err != nil {
        return nil, wrapError(err)  // Returns typed error
    }

    return &asyncClient{session: session, ...}, nil
}
```

**2.2 Error Classification**
```go
func classifyError(err error) error {
    // Parse SDK errors, network errors, HTTP status codes
    // Return typed error: InfrastructureError | ProtocolError | AuthError
}

func isProtocolError(err error) bool {
    // Only return true for logical protocol mismatches
}
```

**2.3 Protocol Detection in `RegisterTools`**
```go
func (h *Handler) RegisterTools(...) error {
    // Probe protocol(s) during registration
    protocols := h.detectProtocols(ctx, serverInfo.SSELink())

    // Save for future use (А2 groundwork)
    serverEntity.UpdateSupportedProtocols(protocols)

    // Use detected protocol for listing tools
    client := h.createClientWithProtocol(ctx, protocols[0], httpClient)
    // ...
}
```

**2.4 Unified Timeout Strategy**
```go
// Use context deadline as-is for both transports
// No per-transport timeout override
session, err := client.Connect(ctx, transport, nil)  // ← Same ctx for both
```

**2.5 Test Strategy**
- Unit tests: Error classification, protocol detection
- Integration tests: Fallback scenarios (Streamable fails → SSE succeeds)
- Concurrent tests: Cache thread-safety with 1000+ clients
- Scenario: Server supports only SSE → fallback works

### Complexity Tracking

**Why this complexity is justified:**

| Aspect | Reason |
| --- | --- |
| Custom error types | Essential for protocol-aware fallback vs retry-based resilience |
| Protocol detection in RegisterTools | Enables future persistence (А2); provides signals for adoption tracking |
| Thread-safety fix | Mandatory for scale (2000 concurrent clients); fallback logic amplifies concurrency |
| Context fix (oauth_refresher) | Prevents hanging token refresh; required for correct error propagation |

**Mitigation:**
- Errors are typed, not nested (clear semantics)
- Protocol detection is isolated to RegisterTools (not every call)
- Thread-safety fix is modular (cache logic)
- Context fix is surgical (one line change)


| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., 4th project] | [current need] | [why 3 projects insufficient] |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient] |

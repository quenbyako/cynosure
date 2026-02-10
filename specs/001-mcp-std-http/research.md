# Research: mcp-std-http

**Feature**: mcp-std-http
**Status**: Completed (with critical architecture findings)
**Last Updated**: 2026-01-23

## Part 1: SDK Analysis & Findings

### Transport Layer: CONFIRMED AVAILABILITY

✅ **FINDING**: MCP SDK v1.2.0 includes **BOTH** transports:

| Transport | Class | Location | Notes |
| --- | --- | --- | --- |
| **Streamable HTTP** | `StreamableClientTransport` | `mcp/streamable.go` | Modern protocol, exponential backoff |
| **SSE** | `SSEClientTransport` | `mcp/sse.go` | Legacy protocol, Server-Sent Events |
| **Command** | `CommandTransport` | `mcp/cmd.go` | For stdio-based servers |

Both implement the `Transport` interface:
```go
type Transport interface {
    Connect(ctx context.Context) (Connection, error)
}
```

### StreamableClientTransport Details

**Constructor parameters:**

- `Endpoint string` - HTTP URL to POST requests and GET streams
- `HTTPClient *http.Client` - Reusable HTTP client (can share auth)
- `MaxRetries int` - Exponential backoff with growth factor 1.5 (default=5, caps at 30s)

**Behavior:**

- Sends client requests via **POST** with JSON body
- Receives responses via **Server-Sent Events (SSE)**
- Built-in retry with exponential backoff (`reconnectGrowFactor = 1.5`, `reconnectMaxDelay = 30s`)
- Auto-resumes streams using Last-Event-ID
- Supports "stateless" mode (no session state on server)

### SSEClientTransport Details

**Constructor parameters:**

- `Endpoint string` - HTTP GET endpoint (same URL as Streamable can accept)
- `HTTPClient *http.Client` - Reusable HTTP client

**Behavior:**

- Sends **GET** request to establish stream
- Receives all messages (request + response) via SSE
- No built-in retry in transport layer (must handle at higher level)

### Critical Architectural Insight

Both transports can use the **SAME URL** (Streamable spec allows fallback):

- Client sends POST (Streamable) to `https://example.com/mcp`
- If POST fails, client can retry with GET (SSE) to same URL
- Server's `StreamableHTTPHandler` accepts both patterns on same endpoint

### Fallback Strategy for Dual Protocol

- **Decision**: Implement `connectWithFallback` in `newAsyncClient`.
- **Approach**: Try Streamable HTTP first, on failure try SSE with same URL/auth.
- **Rationale**: Encapsulation at adapter layer; no caller-level awareness of transport strategy.

## Part 2: Scale & Concurrency Analysis

### Current Architecture Review

**Connection Management:**

- Cache with LRU + TTL: `cache.New(constructor, destructor, maxSize=5, ttl=10*time.Minute)`
- **Critical Issue**: maxSize=5 is a hard limit on concurrent connections per `Handler` instance.
- Each connection is `*asyncClient` holding `mcp.ClientSession` + context cancel.
- Sessions use SSE (long-lived HTTP streaming)

**Connection Lifecycle:**

- `RegisterTools`: Creates temp client, lists tools, saves to DB, then closes client (no caching)
- `ExecuteTool`: Gets client from cache by `accountID`; if not cached, creates via constructor
- `RetrieveRelevantTools`: Does NOT create connections; reads from cached account metadata only

### Scale Analysis: 1000 SSE + 1000 HTTP Clients

#### Finding 1: Current Architecture CANNOT Support 2000 Concurrent Clients

```text
Problem:
- Cache maxSize=5 means max 5 active connections per Handler
- If system spawns 1000+ requests simultaneously, 995+ will queue on cache.Get()
- SSE connections: Each opens persistent HTTP/1.1 stream (expensive resource)
- HTTP (non-streaming): Lower resource cost but still requires file descriptors
```

**Resource Impact for 2000 Clients:**

| Resource | SSE (Long-Lived) | HTTP (Streaming) | Issue |
| --- | --- | --- | --- |
| File Descriptors | 2000 FDs | 2000 FDs | OS limit (ulimit -n, typically 1024-4096) |
| Memory per connection | ~50-100 KB | ~20-50 KB | 2000 × 75 KB ≈ 150 MB (manageable) |
| Context goroutines | 1 per client | 1 per client | Bounded; likely OK |
| TCP connections | 2000 | 2000 | Network capacity; ISP limits |
| Keepalive overhead | SSE sends keepalive every 10s | Depends on impl | Network I/O cost |

**Verdict**: 2000 concurrent clients is **FEASIBLE** if:

1. Cache `maxSize` is increased significantly (e.g., 10000+)
2. File descriptor limit is raised (`ulimit -n 65536`)
3. MCP server can accept 2000 concurrent streams
4. Network bandwidth available

### Scale Analysis: Do We Need Long-Lived HTTP?

#### Finding 2: Long-Lived HTTP Likely REQUIRED

**Evidence from codebase:**

- `RegisterTools` creates SHORT-LIVED temp client; never reused
- `ExecuteTool` ALWAYS calls `h.clients.Get()` → retrieves cached or creates new
- System supports 1000s of accounts per user (implied by brainstorm.md)
- Each account = separate ServerID → separate connection
- MCP spec: Tool calling requires bidirectional message exchange

**Hypothesis**: MCP tool execution may require:

1. Tool call submitted to server
2. Server responds with result (or streams result for long-running tasks)
3. Client waits for completion

If tools are truly "long-running jobs," a single request-response HTTP call is insufficient. SSE or WebSocket is needed.

**Assumption**: Streaming HTTP transport (like SSE) is **REQUIRED** for all connections, not just registration.

### Concurrency Pattern Risk

#### Finding 3: Cache Contention is Underspecified

From `cache.go`:

```go
// TODO: VERY VERY IMPORTANT: method is not thread-safe.
```

**Risk**: The `Cache.Get()` method has a known thread-safety bug.

**Impact**: With 1000s of goroutines calling `ExecuteTool` in parallel, race conditions are LIKELY. This is a **blocking issue** for scale.

## Part 3: Critical Plan Review & Gaps

### Gap 1: Plan Ignores Scale Implications

**Current Plan States:**

- "Adapter-level change"
- "maxSize=5"
- "N/A (Stateless connection logic)"

**Reality:**

- Adding fallback logic does NOT address the maxSize=5 bottleneck
- Plan does not propose cache configuration changes
- No analysis of how many connections will be needed

**Recommendation**: Plan MUST include:

- Decision on cache `maxSize` based on expected concurrent accounts
- Configuration guidance for operators

### Gap 2: Plan Does Not Address HTTP Transport Availability

**Current Plan:**

- Assumes `mcp.StreamableHTTPTransport` exists
- Does not provide fallback if transport unavailable

**Recommendation**: Plan MUST include:

- Investigation task to verify SDK support
- Alternative if SDK lacks HTTP transport (DIY implementation vs. upgrade SDK version)

### Gap 3: Spec Missing Edge Cases Around Long-Running Tasks

**Current Spec Edge Cases:**

```text
- What happens when the URL is invalid?
- How does system handle auth errors?
```

**Missing Edge Cases:**

- What if a tool call takes 30+ seconds? Does fallback timeout work?
- What if HTTP transport times out mid-request? Should we keep SSE connection warm?
- What if Streamable HTTP is slower than SSE for small calls? Should we have protocol hints?

**Recommendation**: Spec SHOULD clarify:

- Timeout behavior for each transport
- Whether transport choice is automatic or configurable

### Gap 4: Plan Does Not Address Cache Thread-Safety Bug

**Issue**: `cache.go` has a "VERY VERY IMPORTANT" TODO about thread-safety.

**Implication**: The fallback logic will inherit a broken cache.

**Recommendation**: Plan MUST either:

- Fix cache thread-safety as a prerequisite
- Explicitly scope this feature as "not for high concurrency" until cache is fixed
- Or ensure fallback logic avoids concurrent cache operations

### Gap 5: RegisterTools Creates & Discards Clients

**Current Behavior**: In `registry.go`, `RegisterTools`:

1. Creates temporary client
2. Lists tools
3. Immediately closes client
4. Never reuses it

**Issue**: If we implement fallback in `newAsyncClient`, the fallback logic will:

- Attempt HTTP transport (may fail if not supported)
- Fall back to SSE
- Then immediately close

**This is wasteful**: We're testing both protocols for a one-off registration.

**Recommendation**: Plan should decide:

- Should registration also respect the fallback strategy? (Yes, for consistency)
- Or should registration use a different, simpler code path? (Maybe, for efficiency)

## Part 4: Plan Assessment Summary

### Strengths

✓ Correctly identifies adapter layer as modification point
✓ Respects constitution (bounded contexts, layers)
✓ Preserves URL consistency (single URL, dual protocol)
✓ Includes test-first approach

### Weaknesses

✗ Does not address cache maxSize=5 bottleneck
✗ Does not verify SDK availability for HTTP transport
✗ Does not document scale implications
✗ Does not address cache thread-safety TODO
✗ Does not clarify behavior of RegisterTools fallback
✗ Missing timeout/timing behavior specification
✗ No guidance on operator configuration

### Architectural Concerns

**Concern 1**: Plan assumes `newAsyncClient` modification is isolated, but:

- Cache configuration is entangled
- Thread-safety affects entire handler
- Token refresh logic may not work with both transports

**Concern 2**: Spec says "reuse same HTTP client (with OAuth)," but:

- OAuth refresher uses `context.TODO()` in current code (see `oauth_refresher.go`)
- Fallback may trigger during token refresh
- Unclear if context propagation works correctly

**Concern 3**: SSE is long-lived; HTTP streaming needs design clarity:

- How long should HTTP stream stay open?
- Should we pool connections or create per-request?
- Current code assumes one connection per account, is that still valid?

## Recommendations for Plan Revision

### Tier 1 (MUST address before implementation)

1. **Verify SDK Support**: Run test against `mcp` v1.2.0 to confirm `mcp.StreamableHTTPClientTransport` (or equivalent) exists.
2. **Cache Configuration**: Update plan to include decision on maxSize (propose: based on peak concurrent accounts).
3. **Thread-Safety**: Document that implementation assumes cache thread-safety fix OR implement lockfree fallback logic.

### Tier 2 (SHOULD address before implementation)

1. **Timeout Behavior**: Spec timeout for HTTP (e.g., 30s) vs SSE (infinite or long-lived).
2. **RegisterTools Strategy**: Decide whether temporary client for registration should use fallback.
3. **Context Propagation**: Update oauth_refresher to use request context instead of `TODO()`.

### Tier 3 (MAY address in Phase 2/3)

1. **Performance Tuning**: Measure fallback overhead; consider hints or config to prefer one transport.
2. **Observability**: Add logging/metrics for transport selection and fallback triggers.
3. **Operator Guide**: Document cache sizing, FD limits, and expected concurrency.

## Conclusion

The plan is **architecturally sound** but **underspecified for production scale**. The fallback mechanism itself is simple and correct; however, it assumes a cache that:

- Is configurable for scale
- Is thread-safe
- Is supplemented with correct context/timeout handling

None of these are currently addressed in the plan. **Recommend:** Revise plan to include Tier 1 tasks before starting implementation.

---

## Part 5: Unanswered Questions from SDK Analysis

After analyzing the MCP SDK v1.2.0 source code, I have identified specific questions that require YOUR domain knowledge to answer. These are implementation details that depend on business logic or system design decisions that I cannot infer from code alone.

### Category A: Protocol Behavior & Selection

**Q-A1**: **When should the fallback to SSE occur?**

- Option A: Always try Streamable first, fall back to SSE on ANY error (connection, protocol, auth timeout)
- Option B: Be selective—fall back only on specific errors (e.g., "connection refused", "protocol not supported") but NOT on auth errors (401, 403)
- Option C: Use HTTP headers or server metadata to detect which transports are supported, and choose accordingly (no trial-and-error)

**Why this matters**: Option B might prevent unnecessary fallback attempts when auth is misconfigured. Option C is more efficient but requires out-of-band discovery.

---

**Q-A2**: **Should the transport choice be sticky or re-evaluated per connection?**

- Option A: Once Streamable succeeds, always use Streamable for that account
- Option B: Re-evaluate on each connection (allows server upgrades)
- Option C: Cache the choice with TTL (e.g., remember for 1 hour, then retry Streamable)

**Why this matters**: This affects cache design and how we handle server updates (e.g., server adds Streamable support after initial SSE fallback).

---

**Q-A3**: **Should Streamable's built-in retry be suppressed in favor of our fallback?**

Current code:
```go
StreamableClientTransport{
    Endpoint:   u.String(),
    HTTPClient: httpClient,
    MaxRetries: 5, // 5 exponential backoff retries built-in
}
```

Should we:
- Option A: Keep MaxRetries=5 (Streamable retries internally, THEN we fall back to SSE)
- Option B: Set MaxRetries=0 (disable Streamable retries, rely on our fallback instead)
- Option C: Make MaxRetries configurable per account/server

**Why this matters**: Option A may delay SSE fallback by 30+ seconds. Option B is faster but loses Streamable's built-in resilience. Option C adds complexity.

---

### Category B: Error Handling & Observability

**Q-B1**: **What should we log/track when fallback occurs?**

Should we:
- Option A: Silent fallback (no logs unless error)
- Option B: INFO level: "Streamable failed, falling back to SSE"
- Option C: Capture and expose metrics (e.g., Prometheus: `mcp_transport_fallback_total`)
- Option D: Track which servers support which transports (for analysis)

**Why this matters**: Operators need visibility into fallback frequency. If 90% of servers need SSE, that's a signal that Streamable adoption is low.

---

**Q-B2**: **If both Streamable AND SSE fail, what error should we return?**

- Option A: Return the Streamable error (primary attempt failed)
- Option B: Return the SSE error (final fallback failed)
- Option C: Wrap both errors: "Streamable failed: X; SSE fallback failed: Y"
- Option D: Return a synthetic error: "No compatible transport available"

**Why this matters**: Operators need to diagnose whether the issue is with the server, network, or protocol support.

---

### Category C: Context & Timeout Handling

**Q-C1**: **Should the fallback timeout be per-transport or per-attempt?**

- Option A: `ctx.WithTimeout(ctx, 30s)` applies to **entire** fallback sequence (Streamable + SSE)
- Option B: Each transport gets its own timeout (30s Streamable, 30s SSE = 60s total)
- Option C: Streamable timeout=10s (fail fast), SSE timeout=30s (take longer)

**Current code** in `handler.go`:
```go
return newAsyncClient(ctx, serverInfo.SSELink(), httpClient)
// ^ ctx is passed as-is, no timeout wrapper
```

**Why this matters**: Option A might fail before SSE even gets a chance. Option B doubles latency. Option C is complex but optimal.

---

**Q-C2**: **How should we propagate context during token refresh?**

**Current code** uses `context.TODO()`:
```go
newRefresher(
    context.TODO(),  // <-- BUG: This is NOT the request context!
    auth,
    accounts,
    session,
    serverInfo,
)
```

Should:
- Option A: Pass request `ctx` instead of `TODO()` (may cause premature cancellation during refresh)
- Option B: Use `context.WithoutCancel(ctx)` (detach from request, but keep request timeout)
- Option C: Create a separate context with custom timeout for token refresh (e.g., 5s)

**Why this matters**: Token refresh must not block tool execution, but also must not ignore cancellation forever.

---

### Category D: Connection Lifecycle

**Q-D1**: **Should RegisterTools also use the fallback strategy?**

Current behavior: `RegisterTools` creates a **temporary** client, lists tools, closes it.

```go
c, err := newAsyncClient(ctx, serverInfo.SSELink(), httpClient)
// ... list tools ...
c.Close()
```

Should:
- Option A: Yes, use fallback (consistent, but wasteful: tests both protocols for one-off call)
- Option B: No, RegisterTools uses simple strategy (SSE only, for speed)
- Option C: Auto-detect: If account cache has successful transport, RegisterTools uses that

**Why this matters**: RegisterTools happens once per account, fallback overhead is minimal. But SSE is legacy, so maybe Streamable is guaranteed?

---

**Q-D2**: **How should we handle long-lived vs short-lived connections?**

Both Streamable and SSE support long-lived streams:
- Streamable: POST request stays open for responses, automatic reconnect with Last-Event-ID
- SSE: GET request stays open indefinitely

Should:
- Option A: Configure KeepAlive differently per transport (e.g., Streamable=30s, SSE=60s)
- Option B: Always use same KeepAlive (current: 10s for both)
- Option C: No KeepAlive needed (let underlying HTTP library handle)

**Why this matters**: KeepAlive affects detection of dead servers. Too short = excessive pings; too long = slow detection of disconnects.

---

### Category E: Configuration & Scalability

**Q-E1**: **What should be the cache `maxSize` for handling 2000 concurrent clients?**

Current:
```go
cache.New(constructor, destructor, maxSize=5, ttl=10*time.Minute)
```

Proposal options:
- Option A: Fixed large size (e.g., `maxSize=10000` for all deployments)
- Option B: Configurable (e.g., `CYNOSURE_CACHE_SIZE=10000` env var)
- Option C: Auto-tuning (e.g., based on available memory or runtime.NumCPU())
- Option D: Per-Server-ID cache (separate cache for each MCP server, smaller maxSize each)

**Why this matters**: This is a scaling knob that operators need to tune. Hardcoding may cause memory exhaustion or throughput loss.

---

**Q-E2**: **Should we address the cache thread-safety TODO before implementation?**

From `cache.go`:
```go
// TODO: VERY VERY IMPORTANT: method is not thread-safe.
```

Should fallback implementation:
- Option A: Fix the cache first (prerequisite, blocks this feature)
- Option B: Document that feature is "single-threaded safe" only
- Option C: Implement fallback logic in a way that avoids the race condition (e.g., avoid concurrent cache access)
- Option D: File a separate bug, proceed with implementation assuming fix will happen

**Why this matters**: At scale (1000s of concurrent goroutines), this could cause data corruption or deadlocks.

---

## Summary of Help Needed

**I can now answer:** ✅
- How to implement the dual-protocol fallback (Streamable then SSE)
- Which SDK classes and methods to use
- That the same URL works for both protocols
- That both transports support OAuth sharing

**I CANNOT answer without your domain knowledge:** ❌
- **A1, A2, A3**: Business logic around when/how to select transports
- **B1, B2**: Observability requirements and error handling strategy
- **C1, C2**: Timeout and context propagation policies
- **D1, D2**: Whether RegisterTools needs fallback, KeepAlive tuning
- **E1, E2**: Deployment configuration and prerequisites

**Recommendation**: Before proceeding to implementation planning, please answer **at least A1, B1, and E1** to unblock task generation. The others can be addressed during implementation or in follow-up phases.

---

## PART 6: YOUR DECISIONS & ARCHITECTURAL CONTEXT

### Decision Mapping (А1-Е2)

**А1 - Fallback Strategy**: Protocol-aware fallback
- Fallback on **first connection** to server
- Fallback on **protocol errors** (unknown response, unexpected EOF)
- **FAIL IMMEDIATELY** on infrastructure errors (connection refused, DNS issues, TLS rejected)
- Distinction: Protocol errors → fallback (recoverable). Infra errors → fail fast (no point retrying).

**А2 - Transport Persistence**: Lay groundwork for persistence
- Add field to Server entity: `PreferredProtocol` (Streamable | SSE | Auto)
- Update `ports.ServerStorage` interface to support this
- Do NOT implement persistence logic yet—just architecture for future use
- This allows operators to eventually: force protocol, remember working protocol, etc.

**А3 - Retry Strategy for Protocol Errors**:
- Disable `StreamableClientTransport.MaxRetries` when protocol mismatch detected
- Set `MaxRetries=0` if Streamable fails with protocol error
- No exponential backoff for logical protocol failures
- Rationale: If server doesn't speak Streamable, retrying won't help

**В1 - Observability**: Skip logging for now
- No structured logs for fallback (defer to Phase 2)
- Keep fallback silent unless it results in final error

**В2 - Error Message**: Synthetic error
- If both transports fail: return `"address is not an MCP server (both Streamable HTTP and SSE failed)"`
- Distinguishable from network/auth errors
- Helps operators debug: is it bad URL or bad server?

**С1 - Timeouts**: Unified timeout strategy
- Same timeout for both Streamable and SSE attempts
- Use request context timeout as-is (don't override per-transport)
- Both must complete within same deadline

**С2 - Context Propagation**: Fix context.TODO()
- **MANDATORY**: Replace `context.TODO()` in `oauth_refresher.go` with proper context
- Use `context.WithoutCancel(ctx)` to detach from request cancellation but preserve timeouts
- Token refresh must not block indefinitely, but must not be cancelled by request

**D1 - RegisterTools Strategy**: Protocol detection at registration
- During `RegisterTools`, attempt to **detect** which transport(s) server supports
- Try Streamable first (fast protocol negotiation)
- Fall back to SSE if Streamable fails
- Record result (А2: lay groundwork for `PreferredProtocol`)
- Rationale: Registration is best time to probe server capabilities once

**D2 - KeepAlive & HTTP Client**: Delegate to HTTP client
- Let `http.DefaultClient` and HTTP/2 manage connection pooling
- **BUT**: Remember SSE requires periodic server pings to detect disconnects
- Current keepalive (10s) is adequate for both protocols
- HTTP/2 provides multiplexing; no need custom tuning

**Е1 - Cache Configuration**: Keep fixed for now
- maxSize=5 remains as-is (not a focus for this feature)
- Defer to separate scaling initiative
- Document in Tier 3 recommendations

**Е2 - Thread Safety**: Fix as prerequisite
- **CRITICAL**: The `cache.go` TODO about thread-safety is a blocker
- Fix MUST happen before or during this feature
- Fallback logic will add concurrent access patterns → higher race condition risk
- Implementation approach: Use sync.RWMutex or sync.Map, add tests

### Architectural Invariants & Design Principles

**1. First-Class Citizen Strategy**
- Streamable HTTP is the **target protocol** (future-first)
- SSE is **Degraded Mode** (backward compatibility)
- Architecture should incentivize migration to Streamable
- Future: Track adoption metrics per-server (protocol_version in Prometheus)

**2. Typed Errors for Transport Layer**
- Create custom error types:
  - `InfrastructureError` (connection refused, DNS, TLS) → fail immediately
  - `ProtocolError` (unknown response, unexpected EOF) → try fallback
  - `AuthError` (401, 403) → update token, don't fallback
- Allows business layer to distinguish error categories

**3. Anti-Corruption Layer (DDD)**
- Transport selection details must NOT leak to domain layer
- `ToolManager` interface stays abstract
- Callers don't know if fallback occurred (unless it fails completely)
- Transport choice is infrastructure concern, not business logic

**4. Concurrency Safety (MANDATORY)**
- Cache must be thread-safe for 2000 concurrent clients
- No race conditions tolerated
- Fix thread-safety bug (Е2) as prerequisite
- Use `sync.RWMutex` or `sync.Map` with comprehensive tests

**5. Observability via Prometheus**
- Metric: `mcp_client_connection_protocol{protocol="streamable"|"sse"}`
- Enables dashboards: "Protocol Adoption Rate" over time
- Helps decide when to deprecate SSE
- Phase 2: Implement after initial fallback works

**6. Idempotency & State Management**
- During fallback from Streamable to SSE:
  - Don't duplicate side-effects (e.g., don't send tool command twice)
  - Session state must be preserved
  - Ensure no message loss during protocol switch
- Consider: Does MCP SDK handle this automatically via session abstraction?

### Implementation Implications

**From А1 (Protocol vs Infra Errors)**:
- Must parse error types from SDK
- Create error classification function:
  ```go
  func classifyError(err error) (protocol | infra | auth | other)
  ```
- Only fallback if protocol error

**From А2 (Persistence groundwork)**:
- Add to domain model Server:
  ```go
  type Server struct {
      // ... existing fields
      SupportedProtocols []string // ["streamable", "sse"]
      PreferredProtocol string   // optional hint for future
  }
  ```
- Interface update: `ServerStorage.SaveProtocolInfo(ctx, serverID, protocols []string) error`

**From А3 (No retry on protocol error)**:
- Disable Streamable retry when protocol error detected
- Don't rely on `MaxRetries=5`; set `MaxRetries=0` on fallback

**From С2 (Context fix)**:
- Refactor `oauth_refresher.go`:
  ```go
  newRefresher(
      context.WithoutCancel(ctx),  // ← Fix: proper context propagation
      auth,
      accounts,
      session,
      serverInfo,
  )
  ```

**From D1 (Register phase detection)**:
- `RegisterTools` flow:
  1. Try Streamable (fast protocol check)
  2. If fails → try SSE
  3. Record which worked
  4. Save to `SupportedProtocols` field (А2)

**From Е2 (Thread safety)**:
- Fix `cache.go` concurrent access
- Add tests for concurrent `Get()` calls
- Use proper locking around LRU mutations

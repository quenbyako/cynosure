# Backwards Compatibility Deep Dive: HTTP+SSE → Streamable HTTP

**Context**: MCP Protocol Revision 2025-11-25 vs 2024-11-05
**Impact on 001-mcp-std-http**: CRITICAL - Changes fallback strategy
**Date**: 2025-01-23

---

## 🎯 The Problem: Two Protocol Versions

### Old Protocol (2024-11-05): HTTP+SSE
```
Architecture:
  Client
    ├─ POST to /sse (or similar)        → send requests
    └─ GET from /sse                    → receive stream

  Server has SEPARATE endpoints for:
    - Sending messages (POST endpoint)
    - Receiving messages (SSE endpoint)

  Response format:
    - SSE stream starts with `endpoint` event
    - Contains metadata about other endpoints

  Key: ASYMMETRIC - different endpoints for different directions
```

### New Protocol (2025-11-25): Streamable HTTP
```
Architecture:
  Client
    └─ POST or GET to SINGLE /mcp endpoint

  Server has ONE unified endpoint:
    - POST request → (response OR SSE stream)
    - GET request → SSE stream

  Response format:
    - Either JSON-RPC response directly
    - Or SSE stream with multiple messages

  Key: SYMMETRIC - same endpoint, content-type determines response format
```

---

## 📋 The Backwards Compatibility Strategy (From Spec)

### For Clients (What We Must Build)

```
Step 1: User provides MCP server URL
        ↓
Step 2: POST InitializeRequest to URL
        Include: Accept: application/json, text/event-stream
        ↓
Step 3a: If POST succeeds (HTTP 2xx)
        ✅ NEW PROTOCOL (Streamable HTTP 2025-11-25)
        Continue with this transport
        ↓
Step 3b: If POST fails with 400/404/405
        ⚠️ PROTOCOL MISMATCH
        Assume OLD PROTOCOL (HTTP+SSE 2024-11-05)
        Try step 4
        ↓
Step 4: GET to same URL
        Include: Accept: text/event-stream
        ↓
Step 5: Receive SSE stream
        First event should be `endpoint` event
        ✅ OLD PROTOCOL CONFIRMED
        Continue with old transport
```

**Critical Detail**: Both attempts use SAME URL!

---

## 🔍 What This Means for Error Classification

### Current Plan (Incomplete):

```go
func newAsyncClient(ctx, url, httpClient) {
    // Try Streamable (new protocol)
    session, err := connectWithTransport(ctx, StreamableTransport, url)

    // Fallback on protocol error
    if err != nil && isProtocolError(err) {
        session, err = connectWithTransport(ctx, SSETransport, url)
    }

    if err != nil {
        return nil, err
    }
    return &asyncClient{session}, nil
}
```

### What It SHOULD Be (With Backwards Compatibility):

```go
func newAsyncClient(ctx, url, httpClient) {
    // Step 1: Try Streamable (new protocol from 2025-11-25)
    streamableSession, streamableErr := connectWithStreamable(ctx, url)

    // Step 2: If POST request fails with specific HTTP codes,
    //         try old protocol (HTTP+SSE from 2024-11-05)
    if streamableErr != nil && isOldProtocolError(streamableErr) {
        oldSession, oldErr := connectWithOldSSE(ctx, url)
        if oldErr == nil {
            return &asyncClient{
                session: oldSession,
                protocol: ProtocolOldSSE,  // Track for future calls
            }, nil
        }
        // Both failed
        return nil, synthesizeError(streamableErr, oldErr)
    }

    // Infrastructure error or success
    if streamableErr != nil {
        return nil, streamableErr
    }

    return &asyncClient{
        session: streamableSession,
        protocol: ProtocolStreamable,  // Track for future calls
    }, nil
}
```

---

## 🎯 Error Classification for Backwards Compatibility

### Which HTTP Status Codes Trigger Fallback?

From spec: **"If it fails with: 400 Bad Request, 404 Not Found or 405 Method Not Allowed"**

```go
func isOldProtocolError(err error) bool {
    // Only these specific HTTP codes indicate old protocol
    // (NOT infrastructure errors like connection refused)

    switch httpStatusCode(err) {
    case 400:  // Bad Request (POST not understood)
        return true
    case 404:  // Not Found (endpoint doesn't exist)
        return true
    case 405:  // Method Not Allowed (POST not supported)
        return true
    default:
        return false
    }
}

func isInfrastructureError(err error) bool {
    // These errors should NOT trigger fallback
    switch err.(type) {
    case net.DNSError:           // DNS failure
        return true
    case tls.RecordHeaderError:  // TLS error
        return true
    case net.OpError:            // Connection refused, timeout, etc
        return true
    default:
        return false
    }
}

func isProtocolError(err error) bool {
    // 5xx errors, malformed response, etc
    return !isInfrastructureError(err) && !isOldProtocolError(err)
}
```

---

## 📊 Decision Tree for Connection

```
User provides URL
      ↓
Try: POST InitializeRequest
     Headers: Accept: application/json, text/event-stream
             Content-Type: application/json
             Body: InitializeRequest JSON
      ↓
   ┌──────────────────────┬───────────────────────┬──────────────────┐
   ↓                      ↓                       ↓                  ↓
HTTP 200-299         HTTP 400/404/405      Network Error         Other Error
(Success)           (Protocol Mismatch)    (DNS/TLS/refused)     (5xx, etc)
   │                      │                     │                   │
   ├─→ Parse as JSON      ├─→ Try old protocol  └─→ FAIL HARD       └─→ FAIL
   │   OR SSE stream      │   (GET to same URL)
   │   (check headers)    │
   └─→ NEW PROTOCOL ✅    ├─→ If GET succeeds
                          │   with SSE + endpoint event
                          │   └─→ OLD PROTOCOL ✅
                          │
                          └─→ If GET also fails
                              └─→ Server error (not MCP)
                                  FAIL with "not an MCP server"
```

---

## 🏗️ Architecture Impact on Our Plan

### What Changes in TASK-2 (Fallback Logic)?

**Old Understanding:**
```
Try Streamable HTTP (2025-11-25)
  ↓
Fallback to SSE (but what SSE? New or old?)
```

**Correct Understanding:**
```
Try Streamable HTTP (2025-11-25)
  ├─ If 400/404/405 → Try Old HTTP+SSE (2024-11-05)
  ├─ If network error → Fail immediately
  └─ If success → Done
```

### What Changes in Error Types (TASK-1)?

**Add new error type:**
```go
type OldProtocolError struct {
    statusCode int
    reason     string
}

// Distinguish between:
// - ProtocolError (5xx, malformed response, unexpected EOF)
// - OldProtocolError (400/404/405 → try old transport)
// - InfrastructureError (network, DNS, TLS)
```

### New Task: Understand Old Protocol Structure

**TASK-0.5 (Research, not implementation):**
```
Need to understand:
1. Old protocol endpoint discovery (where is POST vs GET?)
2. Old SSE endpoint format
3. Old message format compatibility
4. How to detect `endpoint` event in SSE stream
```

---

## 🔐 Security Implications

### DNS Rebinding Attack Protection

From spec warning:
> "When implementing Streamable HTTP transport:
> Servers MUST validate the `Origin` header on all incoming connections"

**For our fallback:**
- If server rejects Origin header → 403 Forbidden
- This is NOT "old protocol" (not 400/404/405)
- This is an infrastructure/auth error → FAIL HARD (don't retry)

```go
if httpStatusCode(err) == 403 {
    // Forbidden (auth/security issue)
    // Do NOT fallback to old protocol
    return nil, err  // Infrastructure error
}
```

### Session Hijacking Protection

From spec:
> "The session ID MUST only contain visible ASCII characters"

**For backwards compatibility:**
- Old protocol may have different session format
- New protocol uses cryptographically secure session IDs
- Need to track which protocol version we're using for validation

---

## 📝 Updated TASK-0: Bearer Token Injection with Backwards Compat

**File**: `internal/adapters/tool-handler/bearer_wrapper.go`

```go
type BearerTokenTransport struct {
    base http.RoundTripper
    getToken func(ctx context.Context) (string, error)
    protocolVersion string  // Track which protocol we're using
}

func (b *BearerTokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
    // Add bearer token
    token, err := b.getToken(req.Context())
    if err != nil {
        // Token refresh failed
        return nil, err
    }

    if token != "" {
        req.Header.Set("Authorization", "Bearer " + token)
    }

    // Add protocol version header (new protocol only)
    if b.protocolVersion != "" {
        req.Header.Set("MCP-Protocol-Version", b.protocolVersion)
    }

    resp, err := b.base.RoundTrip(req)

    // Log for debugging backwards compat
    if resp != nil && (resp.StatusCode == 400 || resp.StatusCode == 404 || resp.StatusCode == 405) {
        b.onOldProtocolDetected(req.URL.String(), resp.StatusCode)
    }

    return resp, err
}
```

---

## 🧪 Test Scenarios for Backwards Compatibility

### Test 1: New Server (Streamable HTTP)
```
Setup: Server supports new protocol
Test:
  POST /mcp with InitializeRequest
  ↓
  Expect: HTTP 200 with JSON or SSE stream
  Result: ✅ Use Streamable HTTP transport
```

### Test 2: Old Server (HTTP+SSE)
```
Setup: Server supports old protocol
Test:
  POST /mcp with InitializeRequest
  ↓
  Expect: HTTP 400/404/405 (endpoint not understood)
  ↓
  GET /mcp with Accept: text/event-stream
  ↓
  Expect: SSE stream with `endpoint` event
  Result: ✅ Use old HTTP+SSE transport
```

### Test 3: Mixed (Server supports both)
```
Setup: Server supports both protocols
Test:
  POST /mcp with InitializeRequest
  ↓
  Expect: HTTP 200 (new protocol preferred)
  Result: ✅ Use Streamable HTTP (new protocol)
```

### Test 4: Not an MCP Server
```
Setup: Server is not MCP
Test:
  POST /mcp with InitializeRequest
  ↓
  Expect: HTTP 400/404/405
  ↓
  GET /mcp with Accept: text/event-stream
  ↓
  Expect: Network error OR no `endpoint` event
  Result: ✅ Fail with "not an MCP server" error
```

### Test 5: Network Error (Don't Fallback)
```
Setup: Network is down
Test:
  POST /mcp with InitializeRequest
  ↓
  Expect: net.ConnectError or timeout
  Result: ✅ Fail immediately (don't try old protocol)
```

---

## 🔄 Protocol Detection and Storage

### TASK-3.5 (Modified): Store Protocol Version

```go
type Server struct {
    // ... existing fields ...

    // Protocol versioning
    PreferredProtocol string  // "streamable" or "old-sse"
    ProtocolVersion   string  // "2025-11-25" or "2024-11-05"
}
```

### Why Store Protocol Version?

1. **Future calls**: Don't need to probe again
2. **Monitoring**: Track adoption of new vs old protocol
3. **Debugging**: Know which protocol was used for issues

### Storage Decision

```go
// When connection succeeds, record it
if protocolVersion == "2025-11-25" {
    server.PreferredProtocol = "streamable"
    server.ProtocolVersion = "2025-11-25"
} else if protocolVersion == "2024-11-05" {
    server.PreferredProtocol = "old-sse"
    server.ProtocolVersion = "2024-11-05"
}

// On next call to same server, use remembered protocol
// But still handle fallback in case server changes
```

---

## 📋 Revised Implementation Plan

### TASK-1 (Error types): UNCHANGED
- ProtocolError (5xx, malformed, EOF)
- InfrastructureError (network, DNS, TLS)
- AuthError (401, 403 auth failures)

### TASK-2 (Fallback logic): MODIFIED
**Add backward compatibility detection:**

```
Try Streamable (2025-11-25)
  │
  ├─ Success (HTTP 2xx) → Use Streamable ✅
  │
  ├─ Old protocol signal (400/404/405) → Try old protocol
  │   │
  │   ├─ Success (SSE with endpoint) → Use old HTTP+SSE ✅
  │   │
  │   └─ Failure → Not an MCP server ❌
  │
  └─ Infrastructure error → Fail immediately ❌
```

**Effort change**: +2 hours (understanding old protocol format)

### TASK-3 (Detection): EXPANDED
```
During RegisterTools:
  1. Probe both protocols
  2. Record ProtocolVersion (string)
  3. Record PreferredProtocol (enum)
  4. Use for next calls to same server
```

**Effort change**: +1 hour

### TASK-0 (Bearer injection): UNCHANGED
- Works identically for both protocols
- Both support Authorization header

---

## ⚠️ Critical Questions for Implementation

### Q1: How to detect old protocol endpoint event?

**Answer from spec**: First SSE event should be `endpoint` event

Pseudo-code:
```go
func parseOldProtocolStream(sseStream io.Reader) error {
    decoder := sse.NewDecoder(sseStream)

    // First event must be endpoint event
    event, err := decoder.NextEvent()
    if err != nil {
        return fmt.Errorf("no SSE events: %w", err)
    }

    if event.EventType != "endpoint" {
        return fmt.Errorf("expected 'endpoint' event, got %q", event.EventType)
    }

    // Parse endpoint data
    // ...
    return nil
}
```

### Q2: What if server supports both protocols but returns different behavior?

**Answer**: Use new protocol by default (it's preferred)

Only fallback if new protocol rejects the request.

### Q3: How to handle session IDs across protocol versions?

**Answer**:
- New protocol: `MCP-Session-Id` header
- Old protocol: May be different (need to check spec)

For now: Treat each protocol's session separately

### Q4: What about Bearer token format across versions?

**Answer**: RFC 7235 (Authorization header) is stable

Both protocols should accept same Bearer token format.

---

## 🎯 Updated Recommendation

### Option B Phase 1 (Week 1) - UPDATED:

```
TASK-P1: Cache fix                          (2-3h)
TASK-P2: Context fix                        (1h)
TASK-0: Bearer injection wrapper            (3-4h)
────────────────────────────────────────────
TASK-1: Error types                         (2h)
TASK-2: Fallback logic + OLD PROTOCOL       (5-6h) ← +2h for backwards compat
TASK-3: Protocol detection + storage        (3-4h) ← +1h for versioning
────────────────────────────────────────────
Total Phase 1: ~20-24 hours (was 18-24)
```

### New Effort: +2 hours for backwards compatibility

**Why worth it:**
- ✅ Support both MCP versions 2024-11-05 and 2025-11-25
- ✅ Works with both new and legacy MCP servers
- ✅ No additional complexity after understanding old protocol

---

## 📚 References

**In MCP Spec 2025-11-25:**
- "Backwards Compatibility" section (what we analyzed)
- "HTTP+SSE transport" old documentation
- "Protocol Version Header" - how servers identify versions

**Not needed for us:**
- Old HTTP+SSE server implementation (someone else owns that)
- SSE standard (already understood in our plan)

---

## ✅ Summary: Key Learnings

### 1. **This is NOT just Streamable ↔ SSE fallback**
It's **Streamable (new) → HTTP+SSE (old)** fallback

### 2. **Error codes matter for detection**
Only 400/404/405 → try old protocol
Other errors → fail immediately

### 3. **Protocol version needs tracking**
Store which version server supports for optimization

### 4. **Bearer tokens work the same**
No OAuth changes needed for backwards compat

### 5. **This increases effort by ~2 hours**
Worth it for real-world compatibility

---

## 🚀 Next Steps

1. ✅ Understand backwards compatibility (you just did)
2. ⏳ Review old HTTP+SSE protocol format (if starting implementation)
3. ⏳ Update TASK-2 description with old protocol fallback logic
4. ⏳ Update TASK-3 to store ProtocolVersion
5. ⏳ Create test cases for both protocol versions

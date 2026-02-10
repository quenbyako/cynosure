# Authorization Analysis: MCP Spec vs Current Implementation

**Date**: 2025-01-23
**Scope**: Compare MCP Authorization Spec (2025-11-25) with current codebase and SDK capabilities

---

## Part 1: Key Features from MCP Authorization Spec

### 1. Core Authorization Model

**What the spec requires:**
- OAuth 2.1 MUST be implemented for HTTP-based transports
- Authorization at **transport level** (not application level)
- Bearer token in Authorization header: `Authorization: Bearer <token>`
- **HTTPS only** for all endpoints

**Current codebase status**: ✅ PARTIAL
- Using `oauth2.NewClient()` in `internal/adapters/oauth/`
- But: No bearer token handling in transport layer
- But: Tokens not included in MCP HTTP requests

---

### 2. Protected Resource Discovery (RFC 9728)

**What the spec requires:**
- Server MUST implement RFC 9728: Protected Resource Metadata
- Two discovery mechanisms:
  1. **WWW-Authenticate header** with `resource_metadata` URL on 401 response
  2. **Well-known URI** at `/.well-known/oauth-protected-resource` or path-specific variant

**Current codebase status**: ❌ NOT IMPLEMENTED
- No 401 response handling for resource discovery
- No Protected Resource Metadata endpoint
- No WWW-Authenticate header construction

**SDK Support**: ✅ FULL
```go
// From oauthex/resource_meta.go:
GetProtectedResourceMetadataFromHeader()  // Parse WWW-Authenticate header
GetProtectedResourceMetadataFromID()      // Construct well-known URI
```

---

### 3. Authorization Server Discovery

**What the spec requires:**
- MCP clients MUST try multiple well-known endpoints in priority order:
  1. OAuth 2.0 metadata: `/.well-known/oauth-authorization-server[/path]`
  2. OpenID Connect: `/.well-known/openid-configuration[/path]`
  3. OpenID Connect (path appending): `/path/.well-known/openid-configuration`

**Current codebase status**: ❌ NOT IMPLEMENTED
- No discovery of authorization server endpoints
- Hard-coded OAuth configuration

**SDK Support**: ✅ FULL
```go
// From oauthex/auth_meta.go:
GetAuthServerMeta(ctx, issuerURL, httpClient) // Tries all well-known endpoints
// Validates PKCE support: code_challenge_methods_supported MUST be present
// Validates Issuer field matches for security
```

---

### 4. Client Registration (3 approaches supported)

**What the spec requires:**

1. **Client ID Metadata Documents** (RECOMMENDED)
   - Client hosts metadata at HTTPS URL as `client_id`
   - Example: `https://app.example.com/oauth/metadata.json`
   - Server fetches and validates metadata

2. **Pre-registration** (OPTIONAL)
   - Static client credentials hardcoded or configured

3. **Dynamic Client Registration** (RFC 7591, OPTIONAL)
   - Client registers with `/registration` endpoint

**Current codebase status**: ❌ NOT IMPLEMENTED
- Using static client credentials
- No metadata document hosting
- No dynamic registration

**SDK Support**: ✅ FULL
```go
// From oauthex/dcr.go:
DynamicClientRegistration() // RFC 7591 support
```

---

### 5. Resource Indicators (RFC 8707) - CRITICAL

**What the spec requires:**
- `resource` parameter MUST be included in BOTH authorization and token requests
- Binds tokens to intended audience
- Prevents token reuse across services

**Format**: Canonical server URI (lowercase scheme and host)
```
&resource=https%3A%2F%2Fmcp.example.com
```

**Current codebase status**: ❌ NOT IMPLEMENTED
- Not tracking which resource tokens are for
- No audience validation
- Risk of token misuse/passthrough

---

### 6. Scope Management Strategy

**What the spec requires:**

Priority order for scope selection:
1. Use `scope` parameter from `WWW-Authenticate` header if present
2. Use all scopes from Protected Resource Metadata `scopes_supported`
3. Implement step-up authorization for scope upgrades (RFC 6750)

**Current codebase status**: ❌ NOT IMPLEMENTED
- No 401 Forbidden responses with insufficient scopes
- No step-up authorization flow
- No scope tracking per resource

---

### 7. Security Requirements (MUST IMPLEMENT)

| Requirement | Status | Notes |
|---|---|---|
| HTTPS only | ❌ | No HTTPS enforcement in code |
| PKCE (S256) | ❌ | SDK supports, not used |
| Bearer token in Authorization header | ❌ | Tokens not added to transport |
| Token audience validation | ❌ | Critical gap |
| Redirect URI validation | ❌ | SDK supports, not integrated |
| Token refresh rotation | ❌ | Not implemented |
| No query string tokens | ❌ | Could be violated |

---

## Part 2: Comparison with Current 001-mcp-std-http Plan

### Gap Analysis

| Feature | MCP Spec | Our Plan | Status |
|---|---|---|---|
| Protocol negotiation (Streamable vs SSE) | Not covered | ✅ Primary goal | Independent |
| OAuth2 bearer token handling | REQUIRED | ❌ Missing | Critical gap |
| Resource metadata discovery | REQUIRED (RFC 9728) | ❌ Missing | Critical gap |
| PKCE authorization flow | REQUIRED | ❌ Missing | Critical gap |
| Resource parameter (RFC 8707) | REQUIRED | ❌ Missing | Critical gap |
| Error handling (401/403 with challenges) | Required | ❌ Missing | Design gap |

### Missing from Our Plan

**High Priority (BLOCKERS for spec compliance)**:
1. Bearer token injection into HTTP requests (TASK-X1)
2. Protected resource metadata discovery (TASK-X2)
3. Authorization server metadata discovery (TASK-X3)
4. PKCE authorization flow (TASK-X4)
5. Resource audience binding (TASK-X5)
6. 401/403 error handling with scope challenges (TASK-X6)

**Medium Priority (RECOMMENDED)**:
1. Scope selection strategy implementation
2. Step-up authorization for insufficient scopes
3. Client ID metadata documents support
4. Dynamic client registration support

**Low Priority (OPTIONAL)**:
1. Token refresh rotation
2. DPoP token binding (RFC 9449)
3. Authorization details (RFC 9396)

---

## Part 3: SDK Implementation Review

### What's Already Built (We Can Use)

The MCP Go SDK v1.2.0 already implements:

#### `oauthex` Package (OAuth Extensions)

**Resource Metadata Discovery** (`resource_meta.go`):
```go
// Parse WWW-Authenticate header and extract resource_metadata URL
GetProtectedResourceMetadataFromHeader(ctx, serverURL, headers, httpClient)

// Build well-known URI and fetch metadata
GetProtectedResourceMetadataFromID(ctx, resourceID, httpClient)

// Result: ProtectedResourceMetadata struct with:
//   - authorization_servers []string
//   - scopes_supported []string
//   - bearer_methods_supported []string
```

**Authorization Server Discovery** (`auth_meta.go`):
```go
// Tries all well-known endpoints in priority order:
// 1. /.well-known/oauth-authorization-server[/path]
// 2. /.well-known/openid-configuration[/path]
// 3. /path/.well-known/openid-configuration
GetAuthServerMeta(ctx, issuerURL, httpClient)

// Returns: AuthServerMeta with:
//   - authorization_endpoint string
//   - token_endpoint string
//   - code_challenge_methods_supported []string (MUST have "S256")
//   - issuer string (validated to match request)
```

**Dynamic Client Registration** (`dcr.go`):
```go
// RFC 7591 support for registering clients dynamically
```

**OAuth2 Flow** (`oauth2.go`):
```go
// Helper functions for:
// - PKCE parameters (code_challenge, code_verifier)
// - Scope handling
// - Resource parameter (RFC 8707)
```

#### `auth` Package (Token Verification)

**Server-side token verification** (`auth.go`):
```go
// TokenVerifier interface: Check token validity
type TokenVerifier func(ctx context.Context, token string, req *http.Request) (*TokenInfo, error)

// Middleware: RequireBearerToken
// - Extracts Bearer token from Authorization header
// - Calls TokenVerifier
// - Returns 401 with WWW-Authenticate header on failure
// - Adds TokenInfo to request context on success
```

**Token Info Extraction**:
```go
type TokenInfo struct {
    Scopes     []string
    Expiration time.Time
    UserID     string       // For session hijacking prevention
    Extra      map[string]any
}

TokenInfoFromContext(ctx) // Extract from request context
```

---

## Part 4: What We Need to Build (vs What SDK Provides)

### Client-Side OAuth Flow (Currently Missing)

**SDK provides**: Raw building blocks and metadata discovery
**We need to build**: Complete client authorization flow

```
Current: Manual OAuth token management (oauth_refresher.go)
Needed:  Full PKCE + resource parameter + token refresh flow
         + scope challenge handling
         + step-up authorization

SDK helps with:
✅ Resource metadata discovery
✅ Auth server metadata discovery
✅ PKCE parameters generation
❓ Full OAuth 2.1 client (may be in oauthex/oauth2.go, need to check)
```

### Bearer Token Injection (Currently Missing)

**SDK provides**: TokenInfo struct and server-side verification
**We need to build**: Client-side token injection in transport

```
Current: OAuth tokens obtained but not added to Streamable/SSE requests
Needed:  Transport layer (StreamableClientTransport, SSEClientTransport)
         wraps HTTP client with Authorization header injection

Not in SDK: We must wrap the transport ourselves
```

---

## Part 5: Architecture Decision: Authorization Impact on Dual-Protocol Strategy

### Key Insight: Authorization is Transport-Independent

**Good news for our plan:**
- Both Streamable HTTP and SSE can use same OAuth token
- Same Authorization server
- Same Protected Resource Metadata discovery
- Fallback doesn't require different auth flow

**New tasks needed:**
1. **TASK-X1**: Implement client-side OAuth 2.1 flow (PKCE, resource parameter)
2. **TASK-X2**: Inject bearer tokens into both transports
3. **TASK-X3**: Implement Protected Resource Metadata discovery (on 401)
4. **TASK-X4**: Handle scope challenges (403 with insufficient_scope)
5. **TASK-X5**: Step-up authorization flow

### Dependency Analysis

```
Current Plan Prerequisite: Fix cache thread-safety (TASK-P1)
Current Plan: Fallback logic (TASK-1 → TASK-2 → TASK-3)

Authorization Requirements:
┌─────────────────────────────┐
│ Client OAuth 2.1 Flow (X1)  │  ← Must happen FIRST
│ - PKCE parameters           │
│ - Resource parameter        │
│ - Token refresh             │
└──────────┬──────────────────┘
           │
           ▼
┌─────────────────────────────┐
│ Bearer Token Injection (X2)  │  ← Wraps both transports
│ - Add Authorization header   │
│ - To Streamable + SSE        │
└──────────┬──────────────────┘
           │
           ▼
┌─────────────────────────────┐
│ Resource Discovery (X3)      │  ← Handle 401 responses
│ - WWW-Authenticate parsing   │
│ - Metadata fetching          │
│ - Auth server discovery      │
└──────────┬──────────────────┘
           │
           ▼
┌─────────────────────────────┐
│ Fallback Logic (TASK-1/2/3)  │  ← Now with auth aware
│ - Error classification       │
│ - Protocol negotiation       │
│ - Protocol preference        │
└─────────────────────────────┘
```

### Impact on Our Current Tasks

| Current Task | Auth Impact | Changes Needed |
|---|---|---|
| TASK-P1: Cache fix | None | No change |
| TASK-P2: Context fix | Affects token refresh | Verify token refresh context |
| TASK-1: Error types | HIGH | Add AuthError classification, 401/403 handling |
| TASK-2: Fallback logic | HIGH | Integrate with auth flow, distinguish auth errors |
| TASK-3: Protocol detection | MEDIUM | Must work with authorization flow |
| TASK-4: Server entity | MEDIUM | Add auth server info, scope info |
| TASK-5: ServerStorage | MEDIUM | Store auth server URL, supported scopes |

---

## Part 6: Recommendations

### Phase 2a: Prerequisites (Unchanged)
- TASK-P1: Cache thread-safety fix ✅
- TASK-P2: Context propagation fix ✅

### Phase 2b: Authorization Foundation (NEW)
- **TASK-X1**: Implement client OAuth 2.1 flow (PKCE, resource parameter)
  - Effort: 4-5 hours
  - Blocker for X2, X3
  - Uses SDK: `oauthex.GetAuthServerMeta()`, PKCE generation

- **TASK-X2**: Bearer token injection in transport layer
  - Effort: 2-3 hours
  - Wraps both StreamableClientTransport and SSEClientTransport
  - Uses SDK: `auth.TokenInfo`

### Phase 2c: Error Handling (MODIFIED)
- **TASK-X3**: Implement Protected Resource Metadata discovery
  - Effort: 2-3 hours
  - Handle 401 responses with WWW-Authenticate header
  - Uses SDK: `oauthex.GetProtectedResourceMetadataFromHeader()`

- **TASK-X4**: Scope challenge handling (insufficient_scope)
  - Effort: 2-3 hours
  - Handle 403 with scope parameter in WWW-Authenticate
  - Step-up authorization flow

### Phase 2d: Dual-Protocol with Auth (MODIFIED EXISTING)
- **TASK-1**: Error types (EXPAND)
  - Current: InfrastructureError, ProtocolError, AuthError
  - New: ScopeError, TokenError, ResourceError
  - Effort: +1 hour

- **TASK-2**: Fallback logic (EXPAND)
  - Current: Streamable → SSE, classify errors
  - New: Integrate auth checks, don't retry on auth errors
  - Effort: +2 hours

- **TASK-3**: Protocol detection (EXPAND)
  - Current: RegisterTools probes both
  - New: Must happen AFTER auth flow established
  - Effort: +1 hour

### Estimated Total Additional Effort
- Authorization implementation: **12-16 hours**
- Modifications to existing tasks: **4-5 hours**
- **New total effort: 38-45 hours** (was 22-32 hours)

---

## Part 7: Decision Point for Team

### Option A: Implement Authorization Fully (MCP Spec Compliant)
**Pros:**
- Spec compliant from day 1
- Works with any MCP server requiring auth
- Secure token handling with resource binding
- Supports all three client registration modes

**Cons:**
- Significant additional complexity (12-16 hours)
- Delays dual-protocol feature by 1-2 weeks
- Requires careful implementation of state management

**Recommendation**: Choose if targeting production use with external MCP servers

### Option B: Implement Dual-Protocol First, Auth Later (Phased)
**Pros:**
- Dual-protocol works quickly (3-4 weeks)
- Auth can be separate feature later
- Reduces risk by separating concerns
- Faster time to market

**Cons:**
- Will need refactoring when auth added
- May build incomplete feature
- Transport layer changes needed in both phases

**Recommendation**: Choose if internal-only MCP servers (pre-configured auth)

### Option C: Minimal Auth Support (Scope Subset)
**Pros:**
- Support basic Bearer token flow
- No PKCE or resource parameter validation
- Simpler, ~4-5 hours
- Works with many servers

**Cons:**
- Not spec compliant
- No protection against token reuse
- Limited scope management
- Security concerns

**Recommendation**: Only if time-constrained and willing to accept security gaps

---

## Conclusion

The MCP Authorization specification (2025-11-25) defines a **complete OAuth 2.1 system** that is largely independent of protocol (Streamable vs SSE). The Go SDK v1.2.0 provides excellent building blocks for the client-side flow.

**Key Finding**: Our dual-protocol fallback strategy (Streamable → SSE) is unaffected by authorization requirements. However, **token injection and scope management must work for both protocols equally**.

**Recommended approach**:
1. Fix prerequisites (P1, P2)
2. Implement authorization flow (X1-X4): **12-16 hours**
3. Implement dual-protocol with auth-aware error handling (modified 1-5): **9-12 hours**
4. Test integration (6-7): **5-7 hours**

**Total: ~30-35 hours for spec-compliant dual-protocol with full authorization**

Or pursue Option B (phased) and do auth as Phase 3 feature.

# Feature Specification: mcp-std-http

**Feature Branch**: `001-mcp-std-http`
**Created**: 2026-01-23
**Status**: Draft
**Input**: User description: "Currently, the service only supports MCP SSE servers, even though this is a legacy protocol. However, we MUST maintain SSE support as some MCP servers still use the old protocol. We need to support both protocols. A decision was made to implement a mechanism at the MCP adapter level (`internal/adapters/tool-handler`) that first attempts to connect to a streamable HTTP server and, if that attempt fails, falls back to SSE. Both connection attempts must use the same URL."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Support Streamable HTTP with Fallback to SSE (Priority: P1)

As a system that connects to MCP servers, I want to attempt connection using Streamable HTTP first and fall back to SSE if it fails, so that I can support both modern and legacy MCP servers with a single configuration.

**Why this priority**: Supports the transition to the new protocol while maintaining compatibility (business requirement).

**Independent Test**:

1. Configure a mock MCP server to support only Streamable HTTP. Ensure client connects.
2. Configure a mock MCP server to support only SSE. Ensure client connects.

**Acceptance Scenarios**:

1. **Given** an MCP server supporting Streamable HTTP, **When** the client connects, **Then** it uses Streamable HTTP transport.
2. **Given** an MCP server supporting only SSE, **When** the client connects, **Then** the Streamable HTTP attempt fails, and it seamlessly connects using SSE.
3. **Given** an invalid/unreachable server, **When** the client connects, **Then** it returns an error after attempting both (or failing fast).

### Edge Cases

- **What happens when the URL is invalid?**: Both attempts will likely fail with infrastructure error (DNS/network failure); system fails immediately without fallback.
- **How does system handle auth errors?**: Both transports handle authentication errors identically. If authentication fails (401/403), client MUST NOT retry with different protocol—this is NOT a protocol mismatch.
- **What happens when server returns 400/404/405 on Streamable attempt?**: This indicates server supports older MCP protocol (2024-11-05). System automatically falls back to legacy HTTP+SSE protocol using same URL.
- **What if both Streamable and SSE fail?**: System returns error: "address is not an MCP server (both protocols failed)".
- **What if tool execution takes 30+ seconds?**: Same unified timeout applies to both transports; long-running tasks supported equally by both protocols.
- **How does system handle network failures during protocol detection?**: Infrastructure errors (connection refused, DNS failure, TLS rejection) trigger immediate failure without attempting fallback—no point retrying different protocol for network issues.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST attempt to connect using Streamable HTTP transport first (MCP Protocol 2025-11-25).
- **FR-002**: If Streamable HTTP connection fails with protocol-specific errors (HTTP 400/404/405), the system MUST attempt to connect using legacy HTTP+SSE transport (MCP Protocol 2024-11-05).
- **FR-003**: The fallback logic MUST use the same target URL for both transports.
- **FR-004**: The system MUST maintain the same authentication session across both connection attempts.
- **FR-005**: The system MUST support backward compatibility with MCP protocol version 2024-11-05 (legacy HTTP+SSE).
- **FR-006**: When receiving HTTP status codes 400/404/405 on Streamable request, the system MUST automatically switch to legacy SSE protocol.
- **FR-007**: The system MUST fail immediately on infrastructure errors (DNS, TLS, connection refused) without attempting protocol fallback.
- **FR-008**: The system MUST inject OAuth Bearer tokens into Authorization headers for both Streamable HTTP and SSE transports when connecting to protected MCP servers.
- **FR-009**: The server MAY omit authentication, in which case the system MUST be able to connect without tokens.
- **FR-009**: The system MUST handle 401 responses by attempting Protected Resource Metadata discovery per RFC 9728.
- **FR-010**: The system MUST detect which protocol(s) each MCP server supports during initial registration and record this information for future optimization.

### Non-Functional Requirements

- **NFR-001**: Protocol detection overhead MUST be less than 100ms per connection attempt.
- **NFR-002**: System MUST support at least 2000 concurrent client connections across both protocols.
- **NFR-003**: The fallback mechanism MUST be transparent to calling code—no protocol-specific logic should leak to domain layer.
- **NFR-004**: System MUST maintain backward compatibility with existing SSE-only MCP servers while prioritizing modern Streamable HTTP protocol.
- **NFR-005**: System SHOULD support OAuth 2.1 authorization for protected MCP servers using Bearer token authentication (RFC 7235).
- **NFR-006**: System SHOULD handle Protected Resource Discovery per RFC 9728 when encountering 401 responses from MCP servers.
- **NFR-007**: All OAuth tokens MUST be transmitted via Authorization header only—NEVER in query strings or cookies.
- **NFR-008**: System MUST use HTTPS for all MCP server communication when OAuth is enabled.

### Key Entities *(include if feature involves data)*

- **Connection Manager**: Responsible for managing the connection strategy and protocol fallback logic.
- **MCP Client**: The client instance connecting to the server using either Streamable HTTP or legacy SSE transport.
- **Server Entity**: Tracks protocol support information (SupportedProtocols, PreferredProtocol) for optimization and monitoring.
- **Transport Error Types**: Classification system distinguishing infrastructure, protocol, and authentication errors to enable intelligent fallback decisions.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Successfully connect to modern MCP servers supporting Streamable HTTP protocol (MCP 2025-11-25).
- **SC-002**: Successfully connect to legacy MCP servers using SSE fallback when Streamable is not supported (MCP 2024-11-05).
- **SC-003**: System successfully connects to OAuth-protected MCP servers with Bearer token authentication.
- **SC-004**: Protocol detection completes in under 100ms during initial connection attempt.
- **SC-005**: System supports 2000 concurrent connections with both protocols without performance degradation.
- **SC-006**: Zero protocol-related errors leak to domain layer—all transport selection handled transparently at adapter level.
- **SC-007**: Backward compatibility verified: 100% success rate with legacy SSE-only servers that existed before protocol upgrade.

## Assumptions & Constraints *(optional)*

### Assumptions

- **ASM-001**: MCP Go SDK v1.2.0 or higher is available with both `StreamableClientTransport` and `SSEClientTransport` implementations.
- **ASM-002**: OAuth token management infrastructure already exists (`internal/adapters/oauth/handler.go` and `oauth_refresher.go`).
- **ASM-003**: Both MCP protocol versions (2024-11-05 and 2025-11-25) accept identical Bearer token formats per RFC 7235.
- **ASM-004**: Operators will configure appropriate file descriptor limits (ulimit) for high concurrency deployments.
- **ASM-005**: MCP servers supporting both protocols prefer Streamable HTTP over legacy SSE.

### Constraints

- **CON-001**: Cache thread-safety issue in `contrib/sf-cache/cache.go` MUST be resolved before deployment to support concurrent connections.
- **CON-002**: Context propagation bug (`context.TODO()` in `oauth_refresher.go`) MUST be fixed to prevent token refresh failures.
- **CON-003**: Protocol fallback must not introduce retry logic—fallback is logical protocol selection, not transient error recovery.
- **CON-004**: Infrastructure errors (DNS, TLS, network) must fail immediately without protocol fallback attempts.
- **CON-005**: OAuth token injection must work identically for both transports to maintain authentication consistency.

## Out of Scope *(optional)*

- **OOS-001**: Full OAuth 2.1 client flow implementation (PKCE, dynamic client registration, scope management)—deferred to Phase 1.1 in phased approach.
- **OOS-002**: Persistent storage of protocol preference per server—architectural groundwork included but persistence logic deferred.
- **OOS-003**: Automatic protocol version negotiation beyond backward compatibility (e.g., future protocol upgrades).
- **OOS-004**: Observability metrics and dashboards for protocol adoption tracking—deferred to Phase 2.
- **OOS-005**: Step-up authorization for insufficient OAuth scopes—part of future OAuth enhancement feature.
- **OOS-006**: Dynamic cache size configuration and auto-tuning—separate scaling initiative.

## Dependencies *(optional)*

- **DEP-001**: MCP Go SDK v1.2.0+ with both transport implementations.
- **DEP-002**: Existing OAuth infrastructure (`internal/adapters/oauth/`) for token management.
- **DEP-003**: Thread-safe cache implementation (prerequisite fix required).
- **DEP-004**: HTTP/2 support in Go standard library for connection multiplexing.
- **DEP-005**: Protected Resource Metadata support in SDK (`oauthex` package) for OAuth discovery.

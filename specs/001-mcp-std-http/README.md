# Feature 001-mcp-std-http: Specification Complete ✅

**Status**: Phase 1 Design Complete | Ready for Phase 2 Implementation

**Generated**: 2025-01-23

---

## Overview

Implement dual-protocol MCP support in cynosure's tool handler adapter. The system will attempt Streamable HTTP (modern) first, falling back to SSE (legacy) only on protocol errors. Infrastructure errors fail immediately without fallback.

**Key Principle**: Streamable = First-Class Citizen, SSE = Degraded Mode

---

## Feature Documents

### 1. [spec.md](spec.md)
**What users need, why they need it, how we'll measure success**

- User stories (4): SSE-only limitation, protocol support gap, operator pain, scale constraint
- Requirements:
  - Functional: Dual protocol, same URL, protocol detection, transparent fallback
  - Non-Functional: Scale 2000 clients, <100ms detection overhead, backward compatible
  - Constraints: Error typing, thread-safety fix prerequisite, context propagation fix
- Success Criteria: Both protocols work, fallback is transparent, no operator intervention

**Deliverables**:
- Dual-protocol connection working for both modern and legacy MCP servers
- Unit test coverage ≥85%
- Performance: <100ms protocol detection overhead at registration

---

### 2. [plan.md](plan.md)
**Technical design, architecture, implementation phases**

- Phase 0: ✅ Completed (SDK analysis, decisions, architectural context)
- Phase 1: Design & Contracts (error types, protocol detection, context fix, domain extension)
- Phase 2: Implementation (fallback logic, error classification, tests)

**Architectural Decisions** (А1-Е2, User-Provided):
- **А1**: Fallback on protocol errors only; infrastructure errors fail immediately
- **А2**: Lay groundwork for protocol persistence (add fields, port signatures)
- **А3**: No retry logic for protocol errors (is logical, not transient)
- **В1**: Skip error logging (deferred)
- **В2**: Synthesized error: "address is not an MCP server (both protocols failed)"
- **С1**: Unified timeouts for both protocols
- **С2**: Fix context.TODO() → context.WithoutCancel(ctx) in oauth_refresher
- **D1**: Protocol detection during RegisterTools
- **D2**: Delegate keepalive to HTTP client
- **Е1**: Cache maxSize=5 stays as-is
- **Е2**: Fix cache thread-safety as prerequisite

**Key Constraints**:
- Protocol-aware fallback (not retry-based)
- Cache thread-safety must be fixed first
- Context propagation must be fixed first
- Unified error handling with typed errors

---

### 3. [research.md](research.md)
**Phase 0 investigation results, SDK analysis, decision capture**

- SDK Analysis: Both transports exist in v1.2.0 (StreamableClientTransport, SSEClientTransport)
- SDK Capability: Same URL works for both, auth shared, minimal transport interface
- Decision Mapping: All 8 decision questions (А1-Е2) with user rationale
- Architectural Invariants: 6 design principles (First-Class Citizen, Typed Errors, Anti-Corruption Layer, Concurrency Safety, Observability, Idempotency)
- Implementation Implications: Concrete coding guidance extracted from decisions

**Key Findings**:
- Cache maxSize=5 bottleneck cannot scale to 2000 clients
- Cache thread-safety bug is race condition risk
- Context.TODO() in oauth_refresher prevents proper timeout propagation

---

### 4. [data-model.md](data-model.md)
**Domain entities and their relationships**

- Server Entity: Protocol awareness fields (SupportedProtocols, PreferredProtocol)
- Ports: ServerStorage interface extensions for protocol info
- No new aggregates required
- No changes to existing domain logic (adapter-level change)

---

### 5. [quickstart.md](quickstart.md)
**Getting started guide for developers**

- Setup instructions
- Key files to modify
- Running tests
- Debugging protocol issues
- Common failure scenarios

---

### 6. [tasks.md](tasks.md)
**Concrete implementation work items with acceptance criteria**

**Prerequisites** (Must complete first):
1. TASK-P1: Fix cache thread-safety bug (2-3 hours)
2. TASK-P2: Fix context propagation in oauth_refresher (1 hour)

**Implementation Tasks** (7 tasks, 21-29 hours):
1. TASK-1: Define transport error types (1-2 hours)
2. TASK-2: Implement fallback logic in handler (3-4 hours)
3. TASK-3: Implement protocol detection in RegisterTools (2-3 hours)
4. TASK-4: Extend Server entity with protocol fields (1 hour)
5. TASK-5: Update ServerStorage port (1-2 hours)
6. TASK-6: Unit tests for error classification (2-3 hours)
7. TASK-7: Integration tests for concurrent fallback (3-4 hours)

**Validation** (2 tasks, 3-4 hours):
1. TASK-8: Code review & architecture validation (1-2 hours)
2. TASK-9: Documentation & runbook (2 hours)

**Total Effort**: 22-32 hours | **Critical Path**: ~13 hours

---

### 7. [checklists/requirements.md](checklists/requirements.md)
**Quality gates and acceptance criteria**

✅ Constitution check: Passed (7/7 criteria)
✅ Functional requirements: Complete (6/6)
✅ Non-functional requirements: Complete (4/4)
✅ Domain model: Valid
✅ Ports/adapters: Aligned
✅ Test strategy: Defined
✅ Risk mitigations: Listed

---

## Decision Matrix

| # | Question | Your Answer | Implementation Impact |
| --- | --- | --- | --- |
| **А1** | Fallback strategy | Protocol errors → fallback; infrastructure errors → fail | Error classification required in handler.go |
| **А2** | Protocol persistence | Lay groundwork only | Add SupportedProtocols field to Server entity |
| **А3** | Retry logic | No retry for protocol errors | Set MaxRetries=0 on protocol mismatch |
| **В1** | Error logging | Skip for now | No log statement in fallback path |
| **В2** | Error message | "address is not an MCP server (both failed)" | Synthesized error in newAsyncClient |
| **С1** | Timeout strategy | Unified timeout for both | Use same context deadline for both transports |
| **С2** | Context fix | context.WithoutCancel(ctx) | Fix oauth_refresher.go line ~X |
| **D1** | Protocol detection timing | At registration | RegisterTools probes both transports |
| **D2** | KeepAlive | HTTP client decides | No per-transport tuning needed |
| **Е1** | Cache scaling | Stay fixed | Don't address maxSize=5 now |
| **Е2** | Cache thread-safety | Fix as prerequisite | TASK-P1 before concurrent tests |

---

## Architectural Principles (From Your Context)

1. **First-Class Citizen Strategy**: Streamable HTTP is primary; SSE is fallback
2. **Typed Error System**: Transport errors classified (Infrastructure/Protocol/Auth) at adapter boundary
3. **Anti-Corruption Layer (DDD)**: Domain layer unaware of protocol details; logic in adapter
4. **Concurrency Safety**: Thread-safe design required for 2000 concurrent clients
5. **Observability**: Prometheus metrics track protocol adoption (Phase 3)
6. **Idempotency & State Management**: Fallback doesn't corrupt server state; protocol choice persisted

---

## Implementation Readiness Checklist

Before starting Phase 2 implementation, verify:

- [ ] All team members reviewed decision matrix (А1-Е2)
- [ ] Architectural principles approved by architecture team
- [ ] Cache thread-safety fix scheduled as prerequisite
- [ ] OAuth context fix scheduled as prerequisite
- [ ] Testing environment supports 500+ concurrent client simulation
- [ ] Code review process understood (errors.go first, handler.go second)
- [ ] Logging strategy confirmed (skip for now per В1)
- [ ] Performance baseline established (registration latency <100ms)

---

## Next Steps (For /speckit.tasks Workflow)

### Ready to Execute
1. ✅ All Phase 1 documents complete
2. ✅ User decisions captured (А1-Е2)
3. ✅ Architectural principles documented
4. ✅ Tasks defined with acceptance criteria
5. ✅ Prerequisites identified (TASK-P1, TASK-P2)

### For Team Leads
- Assign TASK-P1 and TASK-P2 as immediate sprint items
- Reserve 22-32 hours of engineering time for Phase 2
- Plan 1-week iteration for completion (assuming 2 engineers)

### For Developers Starting TASK-1
- Read [research.md](research.md) "Decision Mapping" section first
- Review [plan.md](plan.md) "Error Type Design" section
- Start with `errors.go` (TASK-1) before `handler.go` changes (TASK-2)
- Reference [tasks.md](tasks.md) for detailed acceptance criteria

### For Operators (Deployment/Runbook)
- Feature is backward compatible (no operator action needed)
- Both protocols will be transparently selected
- Watch error logs for "address is not an MCP server" if server misconfigured
- Future: Protocol preference will be persisted per-server

---

## Files Modified During This Session

```
specs/001-mcp-std-http/
├── spec.md                   ← Feature specification
├── plan.md                   ← Technical design (updated)
├── research.md               ← Phase 0 investigation (updated)
├── data-model.md             ← Domain entities
├── quickstart.md             ← Developer quickstart
├── tasks.md                  ← Implementation tasks (NEW)
└── checklists/
    └── requirements.md       ← Quality gates
```

---

## Questions Before Starting Implementation?

Review these resources:

1. **"Why these error types?"** → See [plan.md](plan.md) "Phase 1.1: Error Type Design"
2. **"Why layered like this?"** → See [research.md](research.md) "Architectural Invariants"
3. **"What's the protocol detection flow?"** → See [plan.md](plan.md) "Phase 2.3: Protocol Detection"
4. **"How do we handle timeout?"** → See [research.md](research.md) "Decision С1"
5. **"What if both protocols fail?"** → See [plan.md](plan.md) "Phase 2.2: Error Classification"

---

## Feature Readiness Summary

| Aspect | Status | Evidence |
| --- | --- | --- |
| **User Stories** | ✅ Complete | spec.md (4 stories, all validated) |
| **Technical Design** | ✅ Complete | plan.md (3 phases, design locked) |
| **Architectural Decisions** | ✅ Complete | research.md (А1-Е2 captured) |
| **SDK Verification** | ✅ Complete | research.md (both transports confirmed) |
| **Error Strategy** | ✅ Complete | plan.md (typed errors designed) |
| **Test Strategy** | ✅ Complete | tasks.md (14 test cases defined) |
| **Implementation Tasks** | ✅ Complete | tasks.md (9 tasks, all sized) |
| **Code Ready** | ❌ Pending | Implementation phase starts here |
| **Unit Tests** | ❌ Pending | TASK-6, TASK-7 |
| **Integration Tests** | ❌ Pending | TASK-7 |
| **Documentation** | ❌ Pending | TASK-9 |

---

**Ready to begin Phase 2 implementation. Recommend starting with TASK-P1 and TASK-P2 in parallel to unblock core work.**

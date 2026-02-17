# TODO

This file tracks the project's roadmap and backlog using the [TODO.md standard](https://github.com/todomd/todo.md).

## 🔴 High Priority
- [ ] **Implement Internal MCP Server** #feature #architecture @dev
  - [ ] Create server for project-internal tools to follow the "Adapter-as-a-Service" pattern.
- [ ] **Grafana Stack Completion** #observability @dev
  - [ ] **Tempo:** Link `TraceID` across Telegram gateway, Gemini adapter, and MCP client for end-to-end visibility.
  - [ ] **Loki:** Implement Trace-Log Correlation using labels for seamless jump from trace to logs.

## 🟡 Medium Priority
- [ ] **Separate Ephemeral Protocol State** #architecture @dev
  - [ ] Decouple `thought_signature` from permanent database history.
  - [ ] Pass protocol-specific data between turns in-memory/context.

- [ ] **Atomic Message Processing** #architecture #reliability @dev
  - [ ] Ensure 100% consistency in `MergeMessagesStreaming` and DTO converters regarding LLM metadata.
- [ ] **Eradicate `panic` usage** #reliability @dev
  - [ ] Refactor remaining `must()` helpers and `otelMessageFromMessage` to return results/errors.
- [ ] **Security: Conditional PII Masking in Traces** #security @dev
  - [ ] Implement a toggle (e.g., `DEBUG_OTEL_MESSAGES`) to mask PII in production while keeping it for local/dev debugging.
- [ ] **Grafana Stack Completion** #observability @dev
  - [ ] **Tempo:** Link `TraceID` across all service boundaries.
  - [ ] **Loki:** Implement Trace-Log Correlation using labels.
  - [ ] **Metrics:** Wire `CYNOSURE_METRICS_ADDR` into environment config.


## 🟢 Low Priority / Technical Debt
- [ ] **SQL Pagination** #technical-debt @dev
  - [ ] Implement pagination for ALL storage methods in `internal/adapters/sql`.
- [ ] **Agent Loop Interruption** #resilience @dev
  - [ ] Implement "Circuit Breaker" for tools in database, not just in runtime, to stop the agent loop immediately upon tool failure.
- [ ] **Strict MCP Typing** #refactoring @dev
  - [ ] Replace `any`/`json.RawMessage` with concrete types in MCP adapters.

---
## ✅ Completed
- [x] **Fix broken `Text()` method calls** (Updated to `Content()`)
- [x] **Fix Trace Role Mapping** (Corrected `MessageUser` mapping in Gemini OTel attributes)
- [x] **Metrics wiring** (Connected `CYNOSURE_METRICS_ADDR` to configuration)
- [x] **Nil guards for TracerProviders** (Added safety in SQL and Gemini adapters)

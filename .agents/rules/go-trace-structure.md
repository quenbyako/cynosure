---
trigger: model_decision
description: Use, when working with observability, including traces, metrics, logs or profiles.
globs: *.go
---

# Go Observability Structure Rules

## 1. Core Philosophy

Observability (traces, metrics, and logs) is a first-class citizen in our codebase, but it must be decoupled from the core business logic. All telemetry infrastructure, OpenTelemetry dependencies, and logger configurations must reside in a dedicated `observability.go` file within each package. 

By isolating observability code, we ensure:
- The business logic remains clean, readable, and free of tracing boilerplate.
- The observability logic is structured in a templated, boilerplate-friendly manner, allowing for automated code generation.
- Changes to our tracing framework or OpenTelemetry APIs do not leak into business domains.

---

## 2. Requirements and Patterns

### Code Review Rule: Observability Isolation
- Every package implementing or modifying telemetry **MUST** contain an `observability.go` file.
- All telemetry imports (`go.opentelemetry.io/otel/...`) must reside **strictly** within `observability.go`.
- Other files in the package (e.g., `service.go`, `handler.go`, `db.go`) **MUST NOT** import any OpenTelemetry libraries or instantiate OpenTelemetry types.

### Code Review Rule: Static Trace and Event Names
- Span and event names **MUST** be compile-time constants or static templates.
- Building span or event names dynamically at runtime (e.g., via string concatenation or `fmt.Sprintf`) is strictly forbidden.

### Code Review Rule: Context Propagation
- In internal Go-to-Go calls, the trace context **MUST** propagate implicitly via the standard `context.Context` argument.
- Never pass trace propagation headers, trace IDs, or span contexts manually between Go functions.
- External boundaries (e.g., Kafka producers/consumers, HTTP clients/servers, gRPC interceptors) are the **only** places permitted to inject or extract trace propagation headers.

---

## 3. Allowed and Forbidden Cases

### Allowed: Telemetry Isolation via `observability.go`

In `observability.go`:
```go
package chatmodel

import (
	"context"
	"go.opentelemetry.io/otel/trace"
)

type observable struct {
	t trace.Tracer
}

type spanCallback struct {
	span trace.Span
}

func (o *observable) startStream(ctx context.Context) (context.Context, *spanCallback) {
	ctx, span := o.t.Start(ctx, "cynosure.ports.chatmodel.stream")
	return ctx, &spanCallback{span: span}
}

func (s *spanCallback) end() {
	if s.span != nil {
		s.span.End()
	}
}
```

In `service.go` (Business Logic):
```go
package chatmodel

import "context"

type Service struct {
	obs observable
}

func (s *Service) ProcessChat(ctx context.Context) error {
	// Call observability helper; no direct OpenTelemetry import or usage
	ctx, span := s.obs.startStream(ctx)
	defer span.end()

	// ... business logic
	return nil
}
```

---

### Forbidden: Importing OTel in Business Logic

In `service.go` (Violation):
```go
package chatmodel

import (
	"context"
	// FORBIDDEN: Direct import of OpenTelemetry in business logic file
	"go.opentelemetry.io/otel" 
)

func (s *Service) ProcessChat(ctx context.Context) error {
	// FORBIDDEN: Direct call to OpenTelemetry API in business logic
	ctx, span := otel.Tracer("chat").Start(ctx, "stream") 
	defer span.End()

	return nil
}
```

---

### Forbidden: Dynamic Span Names

In `observability.go` (Violation):
```go
func (o *observable) startTransaction(ctx context.Context, actionType string) (context.Context, *spanCallback) {
	// FORBIDDEN: Span name constructed dynamically at runtime
	spanName := fmt.Sprintf("cynosure.transaction.%s", actionType) 
	ctx, span := o.t.Start(ctx, spanName)
	return ctx, &spanCallback{span: span}
}
```

In `observability.go` (Allowed):
```go
// ALLOWED: Static span name
const spanNameTransaction = "cynosure.ports.transaction.execute"

func (o *observable) startTransaction(ctx context.Context) (context.Context, *spanCallback) {
	ctx, span := o.t.Start(ctx, spanNameTransaction)
	return ctx, &spanCallback{span: span}
}
```

---

### Forbidden: Manual Trace Propagation internally

In `service.go` (Violation):
```go
// FORBIDDEN: Passing trace ID or span context instead of standard context.Context
func (s *Service) CalculateBilling(ctx context.Context, traceID string) {
	// ... business logic
}
```

In `service.go` (Allowed):
```go
// ALLOWED: Using standard context.Context for implicit trace propagation
func (s *Service) CalculateBilling(ctx context.Context) {
	// ... business logic
}
```

---

## 4. AGENT ENFORCEMENT CHECKLIST

During code reviews or before proposing code modifications:

1. **Has any tracing, logging, or metrics logic been added/modified?**
   - Verify that all changes are contained within a dedicated `observability.go` file.
2. **Do any files other than `observability.go` import `go.opentelemetry.io/otel/...`?**
   - If yes, reject the change. All OTel interactions must be moved to `observability.go`.
3. **Are span or event names built dynamically?**
   - Check for string concatenation (`+`) or formatting (`fmt.Sprintf`, `fmt.Sprint`) when defining span/event names. Span/event names must be compile-time constants.
4. **Is trace propagation handled manually between Go functions?**
   - Ensure `context.Context` is the sole mechanism for propagating trace contexts within the application. Verify that manual injection or extraction of carrier headers is limited to boundary network adapters.
5. **Does the structure of `observability.go` support templating?**
   - Verify it exposes a clean, boilerplate-free API to the business logic, using wrappers such as `spanCallback` or `eventBuilder`.

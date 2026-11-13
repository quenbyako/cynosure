---
trigger: model_decision
description: Ensures explicit injection of observability dependencies (tracers, meters, loggers, registries), forbidding global singletons or implicit registers.
globs: *.go
---

# Go Explicit Observability Rules

## 1. Core Philosophy

Observability must be explicit, transparent, and completely decoupled from global state. We **NEVER** use global singletons or implicit registration packages from any telemetry or metrics libraries. Using global registries or singletons obscures how telemetry data flows and makes components difficult to test, mock, or isolate.

Every adapter, wrapper, controller, or usecase requiring observability must receive its dependencies (e.g., tracer providers, meter/metric providers, loggers, metric registries, or unified observability stacks) explicitly via constructors (Dependency Injection) or direct function arguments.

---

## 2. Requirements and Patterns

### Code Review Rule: Explicit Dependency Injection
- Observability providers, collectors, and components **MUST** be explicitly passed to constructors or structures as interfaces (e.g., tracer/meter interfaces, registry interfaces, or unified observability stack structures).
- Accessing global tracing or metrics instances (e.g., library-wide global providers, default registries, or default loggers) is strictly forbidden.

### Code Review Rule: No Implicit/Auto Registries
- Never use implicit registration helpers that automatically register metrics or logs to a global/default registry or provider behind the scenes.
- Telemetry collectors and metrics **MUST** be registered to an explicitly passed registry/provider instance.

### Code Review Rule: Refactoring Implicit Telemetry
- If you encounter legacy code or libraries utilising implicit global states, singletons, or implicit registers, they **MUST** be refactored to accept explicit providers rather than being left as-is or deleted.

---

## 3. Allowed and Forbidden Cases

### Allowed: Explicit Injection of Observability Providers

```go
package db

import (
	"context"
)

// Observability interfaces representing generic providers
type TracerProvider interface {
	Tracer(name string) Tracer
}

type MeterProvider interface {
	Meter(name string) Meter
}

type Database struct {
	tracer Tracer
	meter  Meter
}

// ALLOWED: Explicitly injecting providers via constructor
func NewDatabase(tp TracerProvider, mp MeterProvider) *Database {
	return &Database{
		tracer: tp.Tracer("cynosure.db"),
		meter:  mp.Meter("cynosure.db"),
	}
}
```

### Allowed: Unified Observability Stack Injection

```go
package chatmodel

import "github.com/quenbyako/cynosure/internal/domains/cynosure/ports"

type Adapter struct {
	obs observable
}

// ALLOWED: Using explicit ObserveStack port interface
func NewAdapter(stack ports.ObserveStack) *Adapter {
	return &Adapter{
		obs: newObservable(stack),
	}
}
```

---

### Forbidden: Using Global Singletons

```go
package db

import (
	"context"
)

type Database struct {}

func (d *Database) Query(ctx context.Context) {
	// FORBIDDEN: Accessing a global/implicit tracer provider singleton
	ctx, span := globaltelemetry.Tracer("db").Start(ctx, "Query")
	defer span.End()
}
```

### Forbidden: Using Implicit Auto-registration

```go
package httpapi

// FORBIDDEN: Defining telemetry metrics using implicit auto-registration functions
var requestCounter = implicitregistry.NewCounter(CounterOpts{
	Name: "http_requests_total",
	Help: "Total number of HTTP requests.",
})
```

### Allowed: Explicit Registration

```go
package httpapi

type MetricRegistry interface {
	Register(collector Collector) error
}

type Metrics struct {
	requestCounter Counter
}

// ALLOWED: Registering metrics to an explicitly passed Registry/Registerer
func NewMetrics(reg MetricRegistry) (*Metrics, error) {
	counter := NewCounter(CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests.",
	})
	if err := reg.Register(counter); err != nil {
		return nil, err
	}
	return &Metrics{requestCounter: counter}, nil
}
```

---

## 4. AGENT ENFORCEMENT CHECKLIST

During code reviews or before proposing code modifications:

1. **Does the code utilise any global singletons or implicit global variables for observability?**
   - Reject the change. Refactor it to inject the dependency explicitly.
2. **Does the code import or use auto-registration libraries?**
   - Reject the change. Use explicit registration via explicitly passed registries or providers.
3. **Are new components using generic interfaces or the project's `ports.ObserveStack`?**
   - Ensure the constructor accepts these as direct arguments and initialises observers cleanly.
4. **Is there legacy code with implicit telemetry?**
   - Refactor it to use explicit providers instead of deleting it or ignoring it.

---

## References & Examples (Specific Libraries)
- **OpenTelemetry SDK Reference**: Avoid using package-level functions from `go.opentelemetry.io/otel` like `otel.Tracer(...)`, `otel.GetTracerProvider()`, etc.
- **Prometheus Reference**: Avoid using auto-registering packages like `github.com/prometheus/client_golang/prometheus/promauto` or using the global registry `prometheus.DefaultRegisterer`.

<!--
Sync Impact Report
Version change: (none previous) → 1.0.0
Modified principles: N/A (initial set)
Added sections: Core Principles, Additional DDD Constraints & Language, Development Workflow & Complexity Governance, Governance
Removed sections: None
Templates requiring updates:
   .specify/templates/plan-template.md
   .specify/templates/spec-template.md
   .specify/templates/tasks-template.md
Deferred TODOs: None
-->

# Cynosure Constitution

## Core Principles

### I. Bounded Context Isolation

Bounded contexts (e.g. `cynosure` and `gateway`) are INDEPENDENT. Domain code
under `internal/domains/<context>` MUST NOT import domain types, services,
aggregates, or entities from another context. Cross-context interaction MUST
occur only via explicitly published contracts, CLI/text interfaces, or
application orchestrations that transform data without leaking foreign domain
invariants. Shared DTO reuse is DISCOURAGED and ONLY permitted in
controllers or adapters with a documented justification in the PR description.
Rationale: Prevents implicit coupling, preserves autonomy, enables eventual
extraction or separate deployment.

### II. Layered Architecture Integrity

Layers:

- Domain (entities, value objects, domain services, aggregates, ports,
located at `internal/domains/<context>/*`),
- Application (composition & use case
orchestration, located at `internal/apps/<context>/*`),
- Presentation (controllers/handlers, located at
`internal/controllers/<context>/*`),
- Infrastructure (adapters implementing ports, located at
`internal/adapters/<context>/*`).

Business logic MUST exist ONLY in domain (entities, domain services,
aggregates). Application layer MAY coordinate multiple domains but MUST remain
logic-free beyond sequencing and mapping. Controllers/adapters MUST NOT
implement business rules, only data translation and invocation. Domain layer
never depends on adapter concrete types—only port interfaces. Rationale:
Enforces testability, replaceability of infrastructure, and clarity of change
impact.

### III. Ports & Adapters Purity

Every integration to specific driver (SQL with specific migrations, MCP, API to
specific service, e.g. Telegram Bot API, Gemini API, Airtable API, OpenAPI or
AsyncAPI-based Kafka structure) MUST be expressed as a port interface inside the
owning domain. Adapters implement secondary (outbound) ports; primary (inbound)
ports define commands/queries consumed by application/presentation. Mixed port
directory is tolerated short-term. Adapters MUST NOT mutate domain state
directly—only via aggregate or entity methods returned by domain services.
Adding business conditionals in adapters is FORBIDDEN (review gate violation).
Rationale: Establishes clear anti-corruption boundaries and preserves domain
ubiquitous language.

### IV. Aggregate Consistency & Event Sourcing Discipline

Aggregates define atomic consistency boundaries and encapsulate transactional
invariants. Aggregate constructors MUST receive required port interfaces and
immutable identity/value objects only. All state changes MUST occur through
aggregate methods that append a domain event (e.g. `pendingEvents`) representing
the intent. Event sourcing is used for traceability and potential replay; events
MUST be small, explicit, and versioned when structure changes. No external
side-effect (network, storage) occurs outside port invocations triggered from
inside aggregate methods. Aggregates MUST be rehydratable from persisted events
or snapshots without hidden adapter logic. Rationale: Guarantees determinism,
facilitates auditing and future CQRS/read model separation.

### V. Test-First, Contracts & Observability

Tests MUST precede implementation for entities, aggregates, and port contracts.
Red-Green-Refactor cycle is enforced. Each new or changed port requires contract
tests (inprocess + transport-level where applicable). CLI/handler behavior MUST
be testable via stdin/stdout/stderr capturing with deterministic text or JSON
output. Structured logging (context keys: domain, aggregate, action, correlation
id) and metrics MUST exist for domain boundary crossings and event application.
Absence of tests or observability artifacts BLOCKS merge. Complexity
justification table REQUIRED for any added abstraction beyond principles.
Rationale: Ensures reliability, accelerates refactoring, and supports production
diagnostics.

For Golang projects, format of tests should be table-driven whenever it is
possible. If a test case requires complex setup or multiple steps, helper functions should be used to encapsulate that logic and keep the test cases clean.

Format of table-driven tests SHOULD be as follows:

```go
import "github.com/stretchr/testify/require"

func TestSomeFeature(t *testing.T) {
    for _, tt := range []struct {
        name       string
        someParam  someType
        otgerParam int
        wantErr    require.ErrorAssertionFunc
    }{{
        name:       "SomeTestCase",
        someParam:  someValue,
        otgerParam: 42,
    }, {
        name:       "FailureCase",
        someParam:  someValue,
        otgerParam: 1234,
        wantErr:    require.Error,
    }} {
        tt.wantErr = noErrAsDefault(tt.wantErr)

        t.Run(tt.name, func(t *testing.T) {
            text := tt.text
            err := s.adapter.UpdateMessage(t.Context(), tt.msgID, text)
            if tt.wantErr(t, err); err != nil {
                return
            }
        })
    }
}

func noErrAsDefault(f require.ErrorAssertionFunc) require.ErrorAssertionFunc {
    if f != nil {
        return f
    }

    return require.NoError
}
```

#### Testing port implementations

For each port interface defined in a domain, there MUST be a corresponding
test suite in `ports/testsuite` that can be embedded into adapter tests to verify compliance with the contract. This ensures that any adapter
implementing the port can be validated against the expected.

Each test suite MUST provide options to configure necessary parameters
(e.g., valid IDs, timeouts) to allow reuse across different adapter tests, as well as helper functions for common setup tasks. Test suite SHOULD provide setup functions to provide adapters ability to mock external dependencies if needed.

## Additional DDD Constraints & Language

Ubiquitous language originates in each domain and MAY leak outward into
adapters/controllers when doing so clarifies intent (OPTIONAL). Tooling
identifiers (e.g., `ToolInfo`, `AccountID`) MUST NOT be mutated outside domain
methods. Introduction of a concept shared by multiple domains REQUIRES an
explicit published contract (protobuf or textual) and MUST NOT create a hidden
shared kernel inside domain directories. Removing or redefining a term is a
MINOR (if additive) or MAJOR (if breaking semantic meaning) constitution change.

Performance/resilience constraints MUST be expressed as domain invariants
(e.g., max messages per conversation) not embedded inside adapters.
Anti-corruption: inbound data MUST be translated into domain value objects
before business logic runs; outbound translation MUST occur at adapter boundary.

## Development Workflow & Complexity Governance

1. Plan Phase: Constitution Check lists gates: isolation, purity, aggregate
   discipline, test-first, observability.
2. Spec Phase: User stories MUST be independently deliverable (one
   aggregate/use-case producing value without others).
3. Tasks Phase: Separate foundational adapter infrastructure from domain
   modeling—do NOT interleave.
4. PR Review: Reviewer MUST confirm no business logic in non-domain layer,
   verify aggregate methods append events, and tests cover new invariants and
   ports.
5. Complexity Gate: Any cross-domain data structure, custom framework, or
   reflection-based adapter MUST include a justification table in the PR
   referencing Principle II & III.
6. Event Changes: Structural event modifications REQUIRE version bump in event
   type name or field compatibility notes and inclusion in release notes.

Violations: A PR with untested domain logic, adapter conditionals replicating
domain rules, cross-domain imports, or silent side-effects is REJECTED until
corrected.

## Governance

Amendment Procedure: Submit PR with (a) proposed wording diff, (b) impact
analysis (affected templates & tests), (c) version bump rationale
(PATCH/MINOR/MAJOR), (d) migration plan for incompatible changes. At least one
maintainer MUST approve. Version increments follow Semantic Versioning applied
to governance: MAJOR for removal/redefinition of a principle, MINOR for adding a
new principle or expanding scope, PATCH for clarifying language only.

Compliance Review: Weekly or before release tag—scan for cross-domain imports,
adapter logic, missing tests for events, and observability coverage.
Non-compliance opens a blocking issue.

Extraction Guarantee: Any domain directory MUST be extractable to a standalone
repository by copying `internal/domains/<context>` with its ports and adding
adapter implementations externally. Discovery of hidden coupling triggers
remediation task.

Ratification & Dates: First adoption recorded here. Future amendments MUST
update `Last Amended` and follow procedure.

**Version**: 1.0.0 | **Ratified**: 2025-11-07 | **Last Amended**: 2025-11-07

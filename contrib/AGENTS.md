# Contrib Layer Context

<description>
This directory contains self-sufficient modules that are designed to be eventually extracted into their own independent repositories.

The main goal of this code is to provide generic, reusable infrastructure, protocols, and utilities while maintaining zero dependency on the project's internal business logic.
</description>

## Local Commands
Since each subdirectory is a potential standalone package, run checks per module:

- **Tests:** `cd ./contrib/[module] && go test ./... -v`
- **Linter:** `cd ./contrib/[module] && golangci-lint run ./...`
- **Generate:** `cd ./contrib/[module] && go generate ./...`

You **MAY** use `go.work` if it will help reduce cognitive complexity, but keep in mind, that `go.work` **MUST NOT** be commited to the repository.

<rules>

## Strict Rules

1. **[MUST] Self-Sufficiency:** Every package in `contrib/` must be designed as if it were in a separate repository. It should not rely on any `internal/` packages.
2. **[MUST NOT] Business Logic:** These modules must never contain domain-specific logic. They are strictly for infrastructure, transport, or generic utilities.
3. **[MUST] Schema Separation:** Code generation for gRPC, REST APIs (OpenAPI), and SQL must always reside in its own dedicated module, separate from the implementation logic. This ensures the API contract remains independent of its realization.
4. **[MUST NOT] Cross-Contrib Coupling:** Dependencies between modules within `contrib/` are strongly discouraged. A dependency is only allowed if there is a compelling reason and it doesn't create a circular reference.
5. **[MUST] Dependency Injection:** All external dependencies must be passed via constructors. No global states or hidden side effects.

</rules>

## Step-by-Step Workflow

When adding or modifying a component in `contrib/`:

1. If it involves a protocol or API, create/update the dedicated schema module first (e.g., `*-openapi` or `*-proto`).
2. Implement the logic in a separate implementation module, depending only on the schema module.
3. Ensure the module has its own `go.mod` to verfiy that the module is truly independent.
4. Apply the standard repository rules: "one screen" function limit, and idiomatic naming. Configuration for repository (e.g. `.github/`, `.golangci.yml`, `Taskfile.yaml`) may be omitted.
5. Provide comprehensive unit tests using Table-Driven patterns.

## Decision Matrix

- - **Scenario:** Need to use a domain entity in `contrib`
  - **Action:** **Prohibited.** Define a generic interface or data structure within the `contrib` module instead.
- - **Scenario:** Adding a new API client
  - **Action:** Create two modules — one for the generated schema and one for the client implementation.
- - **Scenario:** Module `A` needs a utility from Module `B`
  - **Action:** Re-evaluate if the utility can be moved to a more generic "common" module or if the dependency is truly necessary.

<anti-patterns>
## Anti-patterns
- **Internal Leaks:** Importing business or domain logic from `internal/...` into `contrib/...`.
- **Monolithic Contrib:** Putting multiple unrelated utilities into a single large package.
- **Manual Edits in Generated Code:** Changing code inside modules marked as generated (e.g., OpenAPI clients).
</anti-patterns>

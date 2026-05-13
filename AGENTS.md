# Global Project Rules

## Architectural Invariants

- Every directory **SHOULD** contain an `AGENTS.md` file describing its purpose and specific rules.

## Project Structure

Each directory MAY be templated like `{resource_name}`. In that case, multiple variants are allowed, e.g. for `cmd/{cmd_name}` both directories `cmd/calendar` and `cmd/tasks` may exists together, and inner rules applies to both directories.

- `./cmd/{cmd_name}/` — all application binaries
  - `main.go` — application entry point
  - `{subcmd}/env.go` — defined environment variables for specific command
  - `{subcmd}/cmd.go` — sub-command implementation: action func
- `./contrib/{module_name}/` — self-sufficient modules That worth of being extracted into separate repo, but not yet. See [./contrib/AGENTS.md]
- `./internal/` — core repository logic. See [./internal/AGENTS.md]
  - `apps/{app_name}/` — application directory. Mostly each application is a DI constructor, to provide domain implementation.
    - `wire.go` — wire DI constructor. Uses DSL based on go (means that it's valid go file). Must have `wireinject` build tag to verify, that wire will never be compiled into binary.
    - `app.go` — `App` struct itself, and methods, that can be called during runtime.
    - `app_adapters.go` — constructors for adapters
    - `app_usecases.go`— constructors for usecases
    - `app_controllers.go` — constructors for controllers
    - `app_constructor.go` — file that contains `New` constructor for `App`
    - `app_logs.go` — constructor for observability layer
  - `adapters/{adapter_name}/` — single implementation of specific set of ports (can implement more than 1 port). Usually adapter relates to specific service, database schema (not JUST database), API schema, infrastructure entity, etc.
    - `datatransfer/` — special sub directory in adapter: data transfer logic (conversion from specific libraries to domain types) **ALWAYS MUST** be in this directory.
  - `controllers/{controller_name}/*` — controller is an entry point from specific channel into domain logic. E.g. there should be controller for telegram, http or grpc server, etc. Each controller **MUST** implement only one entry-point (e.g. controller that implements NATS + Kafka is an anti-pattern)

  - `domains/{domain_name}/aggregates/*`
  - `domains/{domain_name}/entities/*`
  - `domains/{domain_name}/ports/*`
  - `domains/{domain_name}/ports/{port_name}/errors.go`
  - `domains/{domain_name}/ports/{port_name}/model.yaml`
  - `domains/{domain_name}/ports/{port_name}/observability.go`
  - `domains/{domain_name}/ports/{port_name}/opts.go`
  - `domains/{domain_name}/ports/{port_name}/port.go`
  - `domains/{domain_name}/ports/{port_name}/testsuite/*`
  - `domains/{domain_name}/ports/{port_name}/testsuite/{port_name}.go`
  - `domains/{domain_name}/ports/{port_name}/testsuite/features/*.- feature`
  - `domains/{domain_name}/ports/{port_name}/testsuite/suite.go`
  - `domains/{domain_name}/ports/{port_name}/wrapper.go`
  - `domains/{domain_name}/primitives/*`
  - `domains/{domain_name}/usecases/*`
  - `logs/*`
  - `logs/semconv/*`
  - `logs/semconv/model/*`


### Directory organising anti-patterns

- **[STRICT] No `pkg` or `utils` directories:** Using `pkg` or `utils` as package names is strictly forbidden. These names are considered junkyard and lead to unorganized code.

## Testing Strategy for Paid APIs

This project uses **API request replay** pattern for integration tests to ensure they are:

- **Deterministic**: Always return the same response for the same input.
- **Offline-capable**: Do not require an internet connection or real API keys in CI/CD.
- **Cost-effective**: Avoid hitting paid API endpoints during routine development.

### Running Tests

By default, tests run in **ReplayOnly** mode using recorded cassettes in the `testdata/` directory.

```bash
go test ./...
```

### Recording/Updating Cassettes

To record new interactions or update expired ones, you **MUST** use the `vcr_record` build tag and provide a real API key:

```bash
go test -v -tags=vcr_record ./...
```

**Note**: The `vcr_record` tag is the ONLY place where `os.Getenv` is permitted in the test suite.

Each package tests will require specific environment variables, e.g. `CYNOSURE_GEMINI_API_KEY`.

> **IMPORTANT:**
>
> By updating ANY of recorded test suite, as an AI agent, you MUST update registry below, to prevent overheaded consumption on token usage for the context.

List of all recorded test suites and env vars that control them:

| directory                   | env vars                  |
| --------------------------- | ------------------------- |
| ./internal/adapters/gemini/ | `CYNOSURE_GEMINI_API_KEY` |

### Cassette Expiration

General policy Cassettes have a TTL of **30 days**. If a cassette is expired, the test will fail with an error. Follow the "Recording" instructions above to refresh them.

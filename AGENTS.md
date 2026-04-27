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

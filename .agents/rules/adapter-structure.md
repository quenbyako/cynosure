---
trigger: glob
globs: internal/adapters/*
---

## Adapter naming

Adapters are named by external component that it using. For example, if adapter connects to Claude API, it **MUST** be named `claude`.

## Structure of adapter

Each type **SHOULD** have next set of basic files:

- `doc.go` — package documentation, also **MUST** contain `const pkgName = "go/package/full/name"`
- `adapter.go` — primary `Adapter` type for whole package.

## Antipatterns

- Adapters **SHOULD NOT** inject other adapters. Imports **SHOULD** be constrained to domain logic and, if necessary, external libraries and SDKs.

  Example:
  ```
  // BAD
  adapters/
    sql/
      import "database/sql"
      import "redis_cache"
    redis_cache/
      import "github.com/redis/go-redis"


  // GOOD
  adapters/
    sql/
      import "database/sql"
    redis_cache/
      import "github.com/redis/go-redis"
  ports/
    database/
      database_port.go
      with_cache.go // combines two ports together
    cache/
      cache_port.go
  ```

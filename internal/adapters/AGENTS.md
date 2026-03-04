# Working with adapters

## Adapter naming

Adapters are named by external component that it using. For example, if adapter connects to Claude API, it **MUST** be named `claude`.

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

- There **SHOULD NOT** be any adapter, that combines multiple PORTS in a single adapter. If it's necessary to combine multiple adapters, it **SHOULD** be implemented as a port utility, not an adapter.

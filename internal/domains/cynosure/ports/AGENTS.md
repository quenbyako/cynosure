# Working with ports

## Common port structure

Common structure of port package is:

- `port.go` — Definition of port + default parameters initialization, if some of port methods provide optional pattern
- `opts.go` — all option utility functions and types that providing optional parameters for port methods. File is optional, and should be created only if there are any options.
- `errors.go` — all errors that port can throw. IMPORTANT: **NEVER EVER** define errors outside of this file
- `wrapper.go` — implementation of wrapper, that contains all observability and other middlewares for ports.
- `factory.go` — API for port implementations (adapters), that will be useful for creating
- `observability.go` — callbacks, related to OpenTelemetry spans/metrics/logs construction.

## Observability

Port **MUST** provide observability tools (such as metrics, tracing, etc.) for each method.

## How to be sure that port is good

### 1. Split between read and write operations

### 2. Define all possible errors, unrelated to port implementation

### 3. Provide test suites for each port

### 4. Provide good documentation for each method

Good documentation MUST contain:

1. What the method does
2. Out of scope usage, if it's not obvious
3. Examples in test suites how does this method works
4. List of errors it can throw. Errors **MUST** be defined, and no any mention of "other error" or "error" should be present. In documentation, mentioned error types **MUST** be writen in square brackets, to point godoc for that type, like this: `[SomeError]`.

   Format **SHOULD** look like this:

   ```go
   // Throws:
   //
   //  - [TypeError], description of error
   //  - [ErrAnother], description of error
   ```

#### Good example of port documentation:

```go
type MessengerClientRead interface {
    // GetUser retrieves full user configuration by ID. May provide not full
    // (forbidden to edit) user in case if user is blocked or didn't verify its
    // email yet.
    //
    // See next test suites to find how it works:
    //
    //  - [TestGetUser] — retrieving user and verifying all fields match
    //    saved values
    //  - [TestBanUser] — shows that user should be disabled if banned.
    //
    // Throws:
    //
    //  - [ErrNotFound], if user doesn't exist.
    //  - [AccessDeniedError], if caller doesn't have access to the user.
    //  - [ErrDisabled], if user exists, but account is disabled due to policy
    //    violation.
    GetUser(ctx context.Context, user ids.UserID) (*entities.User, error)
}
```

#### Bad practices:

1. DO NOT write implementation details in documentation
2. DO NOT write documentation for method more than 5 lines, excluding examples and errors definition
3. DO NOT define only common errors (not provded error means it should never happen)
4. DO NOT define generic term "error", instead, provide EXACT error type or variable.
4. DO NOT mention that there are no test suites for this method, instead propose to create them.
5. DO NOT define implementation specific info.

   - Note: documentation MAY contain tips and recommendations for implementers, but must not define implementation constraints.

## Antipatterns

- Implementation details **MUST NOT** be written in documentation. Even specific implementations **SHOULD NOT** be mentioned.
- Port definition **MUST NOT** contain meta-methods, e.g. health check, ping, etc. These methods **MUST** be defined in constructor of adapter, and port definition DOES NOT responsible for health status of specific adapter implementation.

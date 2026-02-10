# Working with ports


## How to be sure that port is good

### 1. Split between read and write operations

### 2. Define all possible errors, unrelated to port implementation

### 3. Provide test suites for each port

### 4. Provide good documentation for each method

Good documentation MUST contain:

1. What the method does
2. Out of scope usage, if it's not obvious
3. Examples in test suites how does this method works
4. List of errors it can throw. Errors MUST be defined, and no any mention of "other error" or "error" should be present.

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

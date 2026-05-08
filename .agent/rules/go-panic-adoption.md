---
trigger: model_decision
description: The panic adoption rule should be followed, when Golang panics are used in the specific logic.
globs: *.go
---

# Golang agent instructions: Panic usage rules.

## 1. Core philosophy

In Go, `panic` is not an exception mechanism for control flow or standard error reporting. Go handles normal errors via explicit `error` return values. A `panic` indicates a critical, unrecoverable programmer error or a catastrophic state that dictates the application must crash immediately.

**Default Action:** Always prefer returning an `error`. Use `panic` only when explicitly matching the "ALLOWED" conditions below.

---

## 2. When `panic` is ALLOWED

### Rule: Initialization Failures (Fail Fast)

Use `panic` during application startup (`main()` or `init()` functions) if a required dependency or configuration is missing/invalid, making it impossible for the application to run.
*   **Example:** Failing to parse a critical configuration file.
*   **Example:** Failing to establish the initial connection to the primary database on boot.

### Rule: The `Must` Pattern

Use `panic` in package-level helper functions prefixed with `Must` that wrap functions returning `(T, error)`. This is idiomatic for global variable initialization where an error implies a compiled-in bug.

*   **Example:** `regexp.MustCompile("^[a-z]+$")`
*   **Condition:** Only use this for static/constant inputs evaluated at startup, never for dynamic runtime user input.

### Rule: Unrecoverable Logical Invariants

Use `panic` when the code reaches a state that is logically impossible unless there is a bug in the code itself (e.g., out-of-bounds index, nil pointer dereference of an internal structure).
*   **Example:** A `switch` statement on a finite enum where the `default` case is reached, indicating unhandled code changes.

### Other rules

Any other usage of `panic` IS EXPLICITLY FORBIDDEN.



## 3. When `panic` is FORBIDDEN

### Rule: Normal Error Handling (User/I/O Errors)

**NEVER EVER** use `panic` for expected runtime failures, bad user input, network timeouts, file I/O errors, or missing records.

*   **Requirement:** Return an `error` as the last return value and let the caller handle it.

### Rule: Inside Reusable Library Code

**NEVER EVER** let a `panic` escape a library package unless it is an explicitly documented `Must` function.

*   **Requirement:** Libraries must return standard errors to give the consuming application the agency to decide how to respond to the failure.

### Rule: Control Flow

**NEVER EVER** use `panic` and `recover` to simulate `try/catch` exception blocks found in languages like Java or Python for standard control flow routing.

### Rule: Goroutine Boundary Leaks

**NEVER EVER** allow a `panic` to occur in a spawned goroutine without a local `recover` mechanism if the application is intended to be resilient (e.g., an HTTP server). A panic in a child goroutine will crash the entire application process, not just the goroutine.

---

## 4. AGENT ENFORCEMENT CHECKLIST

Before generating or approving Go code, evaluate:

1. Is the code handling user input or external I/O? -> **MUST return `error`.**
2. Is the error happening at runtime after successful startup? -> **MUST return `error`.**
3. Is this a shared library package? -> **MUST return `error`.**
4. Does this failure mean the code itself is fundamentally broken and cannot safely proceed? -> **MAY `panic`.**

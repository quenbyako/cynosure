---
trigger: model_decision
description: Use this rule, when any types or structs were modified or added new
globs: *.go
---

# Compile-Time Interface Implementation Check

## 1. Core Philosophy

In Go, interfaces are satisfied implicitly. While this provides flexibility, it can lead to silent compilation successes that hide incomplete implementations, especially when refactoring interface definitions.

To ensure type safety, prevent runtime errors, and provide a clear visual indicator to anyone reading the code, **every concrete type that implements one or more interfaces MUST have a compile-time verification check declared directly below its definition**.

---

## 2. Requirements and Patterns

### Code Review Rule: Location of the Check

- The verification check **MUST** be placed immediately below the struct or type definition, before any methods or constructors are defined.
- Do not hide verification checks inside tests or at the end of files.

### Code Review Rule: The Pattern

Use the blank identifier pattern, casting a nil pointer of the concrete type to the target interface type:

```go
type Adapter struct {
	// ... struct fields
}

var _ ports.MyPort = (*Adapter)(nil)
```

### Code Review Rule: Multiple Interfaces

If a type implements multiple interfaces, each interface **MUST** be checked explicitly on a separate line immediately below the type definition.

```go
type MultiAdapter struct {
	// ... struct fields
}

var _ ports.Reader = (*MultiAdapter)(nil)
var _ ports.Writer = (*MultiAdapter)(nil)
```

---

## 3. Allowed and Forbidden Cases

### Allowed:

- Declaring checks for custom domain types, adapters, and standard library interfaces (e.g., `io.Reader`, `http.Handler`).
- Grouping interface checks together directly under the struct.

### Forbidden:

- **NEVER** skip compile-time checks for structs that are designed to implement ports, adapters, or key application interfaces.
- **NEVER** use real instances or non-nil allocations for these checks (e.g., do not write `var _ ports.MyPort = Adapter{}` if it allocates memory or performs initialization). Always use `(*Type)(nil)`.
- **NEVER** place these checks inside package init functions or test files if the type is declared in the production codebase.

---

## 4. AGENT ENFORCEMENT CHECKLIST

During code reviews or before proposing code modifications:

1. **Has a new struct been added?** -> Check if it implements any interfaces.
2. **Has an existing struct been modified to satisfy a new interface?** -> Add a compile-time check for that interface.
3. **Is the compile-time check placed directly beneath the struct definition?** -> Ensure it is not placed at the bottom of the file or inside another file.
4. **Does the check use the correct syntax (`var _ Interface = (*Struct)(nil)`)?** -> Verify syntax correctness.

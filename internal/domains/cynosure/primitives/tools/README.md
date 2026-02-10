# Tools Primitives

Domain primitives for tool management in the Cynosure system.

## Overview

The tools package provides value objects for managing AI model tools across multiple accounts. The main challenge it solves is multi-account tool management - language models typically don't understand multi-accounting scenarios well.

## Core Primitives

### RawToolInfo (Value Object)

Represents a single tool that can be executed across one or more accounts.

**Key responsibilities:**
- Maintains tool metadata (name, description, schemas)
- Maps tool IDs to account descriptions
- Adapts schemas for multi-account scenarios
- Selects the appropriate account when processing requests

**Invariants:**
- Tool name and description cannot be empty
- Must be associated with at least one account
- Tool ID slug must match tool name
- Account slug cannot be empty

### Toolbox (Value Object)

An immutable collection of tools indexed by name.

**Key responsibilities:**
- Manages a collection of tools indexed by name
- Handles tool merging when the same tool exists for multiple accounts
- Routes tool invocation requests to the appropriate tool

**Invariants:**
- Tools map cannot be nil
- All contained tools must be valid

## Usage Patterns

### Creating Tools

```go
tool, err := NewRawToolInfo(
    "send_message",
    "Send a message to a chat",
    paramsSchema,
    responseSchema,
    WithMergedTool(toolID1, "Main production bot"),
    WithMergedTool(toolID2, "Backup bot"),
)
```

### Merging Tools

```go
toolbox := NewToolbox()
toolbox, err = toolbox.Merge(tool1, tool2)
```

### Converting Requests

```go
// Toolbox delegates to appropriate RawToolInfo
toolID, params, err := toolbox.ConvertRequest("send_message", modelRequest)
```

# tg-helper Development Guidelines

Auto-generated from all feature plans. Last updated: 2025-11-09

## Active Technologies

## Project Structure

### `cmd/`

### `contrib/`

### `docker/`

### `internal/`

#### `internal/adapters`

#### `internal/apps`

#### `internal/controllers`

#### `internal/domains`

## Code Style

Go 1.25.1 (from go.mod): Follow standard conventions


## Code generation

Use `go generate ./...` from the project root to run all code generation.

Project contains multiple submodules, and `./...` WILL NOT generate submodules.
Submodules stored in `contrib/`, so ensure that code generation is run there as
needed.

## IDE and file editing

It is STRICTLY PROHIBITED to use `rm` command to delete files. Always use IDE tools. All `rm` commands will be rejected by default.

It is also STRICTLY PROHIBITED to use `cat` command to create or edit files. Always use IDE tools. All `cat > ./file << EOF` commands will be rejected by default.

## Domain Model Governance

**CRITICAL RULE**: Changes to `internal/domains/*` require formal approval process.

### Domain Change Request Process

1. **When domain changes needed**: If any task/feature requires modifications to domain model:
   - **DO NOT** modify `internal/domains/*` files directly
   - **CREATE** Domain Change Request document first

2. **Domain Change Request Format**:
   ```markdown
   # Domain Change Request: [Title]

   **Requester**: [adapter-expert | application-expert | your-role]
   **Context**: [cynosure | gateway | ...]
   **Type**: [new-entity | modify-entity | new-aggregate | modify-port | new-usecase]
   **Priority**: [critical | high | normal | low]

   ## Business Justification
   [Why is this change needed from a business perspective?]

   ## Proposed Changes
   [Detailed description of domain model changes]

   ## Affected Components
   - Entities: [list]
   - Aggregates: [list]
   - Ports: [list]
   - Usecases: [list]

   ## Invariants Preservation
   [How existing invariants are preserved or why changes are safe]

   ## Alternative Considered
   [What alternatives were evaluated and why rejected]
   ```

3. **Approval workflow**:
   - Save request to `specs/[feature-number]-[feature-name]/domain-requests/[request-name].md`
   - Invoke `domain_expert` agent using `runSubagent` tool
   - Wait for approval decision document
   - Only implement after receiving approval

4. **Rationale**:
   - Domain model is the core business logic
   - Requires special governance to maintain integrity
   - Prevents infrastructure leakage and broken invariants
   - Ensures domain-driven design principles are followed

**Reference**: See `.github/agents/domain_expert.agent.md` for complete domain governance rules.

<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->

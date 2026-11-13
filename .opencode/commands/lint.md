---
description: Run golangci-lint on the codebase and suggest fixes
agent: quality-coder
---
We are paying down technical debt using the "Double Isolation" strategy. 

Here is the high-level summary of linter issues across the codebase:

!`./scripts/golangci.nu stats`

Choose a SINGLE package with the biggest count of issues, and concentrate ONLY on this package. **DO NOT FIX OTHER PACKAGES EXCEPT THE CHOSEN ONE.**

### Current Target Package & Tier:

Identify the package and linter Tier you are going to address first. Do not run the linter on the whole project!

To see detailed issues for your target package, run:
`./scripts/golangci.nu list -d <TARGET_PACKAGE_PATH> --limit 50`

### Instructions:

1. Examine the issues in the target package.
2. Fix Tier 1 (Mechanical) issues first in a clean, predictable manner.
3. For Tier 2 & 3 issues, plan your changes to preserve architectural patterns (especially Functional Options and clean Error Wrapping).
4. Run tests and verify there are no compilation errors or test failures.


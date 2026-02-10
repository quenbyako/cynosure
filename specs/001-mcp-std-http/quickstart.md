# Quickstart: Testing mcp-std-http

## Prerequisites

- Go 1.25+ installed.
- Access to the repository.

## Running Tests

To verify the fallback logic:

```bash
go test ./internal/adapters/tool-handler/... -v
```

## Manual Verification

1. Start an MCP server that supports only SSE.
2. Run the application (or a test script) pointing to that server.
3. Verify connection succeeds (logs should indicate fallback or successful connection).
4. Start an MCP server that supports Streamable HTTP.
5. Verify connection succeeds (logs should indicate primary transport usage).

# Data Model: mcp-std-http

**Feature**: mcp-std-http

## Overview

This feature does not introduce new persistent entities. It modifies the runtime behavior of the `tool-handler` adapter.

## Runtime Structures

### AsyncClient

Wrapper around `mcp.Client`.

- **Transport**: `mcp.ClientTransport` (Interface, currently `SSEClientTransport`).
- **Session**: `mcp.ClientSession`.

### Handler Strategy

The `Handler` determines the connection method.

- **Primary**: Streamable HTTP (Standard HTTP/POST based JSON-RPC).
- **Fallback**: SSE (Legacy Server-Sent Events).

package mcp

import (
	"context"
	"net/http"
	"net/url"
)

// Probe checks if the specified URL points to a valid MCP server.
// It attempts a raw connection and handshake without performing any data
// operations.
func (h *Handler) Probe(ctx context.Context, u *url.URL) error {
	httpClient := http.DefaultClient

	// Use 0 (invalid/unknown) as preferred protocol to force probing
	client, err := newAsyncClient(ctx, u, httpClient, 0, h.tracer)
	if err != nil {
		return MapError(err)
	}
	defer client.Close()

	// Handshake happened during newAsyncClient creation.
	// If it reached this point, the server is a valid MCP server.
	return nil
}

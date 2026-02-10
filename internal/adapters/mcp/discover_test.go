package mcp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/quenbyako/cynosure/internal/adapters/mcp"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

func TestDiscoverTools(t *testing.T) {
	// 1. Create a mock MCP server that speaks SSE and JSON-RPC
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			// SSE Setup
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.WriteHeader(http.StatusOK)

			// Send endpoint event pointing back to this server for POSTs.
			// Currently, the client uses the same URL for POST if no specific endpoint is given,
			// or we can explicitly send it.
			// Let's send the server URL as the endpoint for messages.
			fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", r.URL.String())
			w.(http.Flusher).Flush()

			// Keep connection open
			<-r.Context().Done()
			return
		}

		if r.Method == http.MethodPost {
			var req map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			id := req["id"]
			method, _ := req["method"].(string)

			var result interface{}

			switch method {
			case "initialize":
				result = map[string]interface{}{
					"protocolVersion": "2024-11-05",
					"capabilities":    map[string]interface{}{},
					"serverInfo": map[string]interface{}{
						"name":    "test-server",
						"version": "1.0.0",
					},
				}
			case "notifications/initialized":
				return // Notification, no response
			case "tools/list":
				result = map[string]interface{}{
					"tools": []map[string]interface{}{
						{
							"name":        "test-tool",
							"description": "A test tool",
							"inputSchema": map[string]interface{}{"type": "object"},
						},
					},
				}
			default:
				// ignore other methods
			}

			if result != nil {
				w.Header().Set("Content-Type", "application/json")
				resp := map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      id,
					"result":  result,
				}
				json.NewEncoder(w).Encode(resp)
			}
		}
	}))
	defer ts.Close()

	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}

	h := mcp.NewHandler(nil, nil, nil)

	t.Run("Valid Account Slug", func(t *testing.T) {
		accID := mustAccountID(t, "valid-slug")
		tools, err := h.DiscoverTools(context.Background(), u, nil, accID, "desc")
		if err != nil {
			t.Fatalf("DiscoverTools failed: %v", err)
		}
		if len(tools) != 1 {
			t.Errorf("expected 1 tool, got %d", len(tools))
		}
		if tools[0].Name() != "test-tool" {
			t.Errorf("expected tool name 'test-tool', got %q", tools[0].Name())
		}
	})

	t.Run("Empty Account Slug", func(t *testing.T) {
		accID := mustAccountID(t, "")
		_, err := h.DiscoverTools(context.Background(), u, nil, accID, "desc")
		if err == nil {
			t.Fatal("expected error with empty slug, got nil")
		}
		// Based on the user report: `tool must be associated with at least one account`
		// and validation error `account slug cannot be empty`.
		// The error comes from RawToolInfo.Validate().
		t.Logf("Got expected error: %v", err)
	})
}

func mustAccountID(t *testing.T, slug string) ids.AccountID {
	uid, _ := ids.NewUserID(uuid.New())
	sid, _ := ids.NewServerID(uuid.New())
	var opts []ids.AccountIDOption
	if slug != "" {
		opts = append(opts, ids.WithSlug(slug))
	}

	aid, err := ids.NewAccountID(uid, sid, uuid.New(), opts...)
	if err != nil {
		t.Fatal(err)
	}
	return aid
}

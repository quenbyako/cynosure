package mcp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/adapters/mcp"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

func noopAccountToken(
	ctx context.Context, id ids.AccountID,
) (entities.ServerConfigReadOnly, *oauth2.Token, error) {
	return nil, nil, nil
}

func noopRefreshToken(_ ids.AccountID, _ *oauth2.Config, token *oauth2.Token, _ bool) (oauth2.TokenSource, error) {
	return oauth2.StaticTokenSource(token), nil
}

func TestDiscoverTools(t *testing.T) {
	mcpURL := setupMTLSTestServer(t)

	handler, err := mcp.New(t.Context(), noopAccountToken, noopRefreshToken,
		mcp.WithUnsafeExternalHTTPClient(http.DefaultTransport),
		mcp.WithInternalHTTPClient(http.DefaultTransport),
	)
	require.NoError(t, err)

	t.Run("Valid Account Slug", func(t *testing.T) {
		accID := mustAccountID(t)
		ctx := context.Background()

		tools, err := handler.DiscoverTools(ctx, mcpURL, accID, "valid-slug", "desc")
		require.NoError(t, err)
		require.NotEmpty(t, tools)
		require.Len(t, tools, 1)
		require.Equal(t, "test-tool", tools[0].Name())
	})

	t.Run("Empty Account Slug", func(t *testing.T) {
		accID := mustAccountID(t)
		_, err := handler.DiscoverTools(context.Background(), mcpURL, accID, "", "desc")
		require.Error(t, err)
	})
}

func setupMTLSTestServer(t *testing.T) *url.URL {
	t.Helper()

	sseChan := make(chan string, 100)
	handler := &testServerHandler{sseChan: sseChan}

	testServer := httptest.NewServer(handler)

	t.Cleanup(func() {
		testServer.CloseClientConnections()
		testServer.Close()
	})

	mcpURL, err := url.Parse(testServer.URL)
	if err != nil {
		t.Fatal(err)
	}

	return mcpURL
}

type testServerHandler struct {
	sseChan chan string
}

func (h *testServerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleSSEConnection(w, r)
	case http.MethodPost:
		h.handleJSONRPCRequest(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (h *testServerHandler) handleSSEConnection(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	if _, err := fmt.Fprintf(w, "event: endpoint\ndata: /rpc\n\n"); err != nil {
		return
	}

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	h.sseEventLoop(r.Context(), w)
}

func (h *testServerHandler) sseEventLoop(ctx context.Context, w http.ResponseWriter) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-h.sseChan:
			if _, err := fmt.Fprintf(w, "event: message\ndata: %s\n\n", msg); err != nil {
				return
			}

			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}
}

func (h *testServerHandler) handleJSONRPCRequest(w http.ResponseWriter, r *http.Request) {
	if !strings.HasSuffix(r.URL.Path, "/rpc") {
		http.NotFound(w, r)
		return
	}

	var req struct {
		Method string `json:"method"`
		ID     int    `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	switch req.Method {
	case "initialize":
		h.respondJSONRPC(w, req.ID, map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"serverInfo": map[string]any{
				"name":    "test-server",
				"version": "1.0.0",
			},
		})
	case "notifications/initialized":
		w.WriteHeader(http.StatusAccepted)
	case "tools/list":
		h.respondJSONRPC(w, req.ID, map[string]any{
			"tools": []any{
				map[string]any{
					"name":        "test-tool",
					"description": "A test tool",
					"inputSchema": map[string]any{"type": "object"},
				},
			},
		})
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (h *testServerHandler) respondJSONRPC(w http.ResponseWriter, id int, result any) {
	resp := struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  any    `json:"result"`
	}{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}

	bytes, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.sseChan <- string(bytes)

	w.WriteHeader(http.StatusAccepted)
}

func mustAccountID(t *testing.T) ids.AccountID {
	t.Helper()

	uid := must(ids.NewUserID(uuid.New()))
	sid := must(ids.NewServerID(uuid.New()))

	return must(ids.NewAccountID(uid, sid, uuid.New()))
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}

	return v
}

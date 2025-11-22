package primitive_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/quenbyako/cynosure/internal/adapters/mocks"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/ids"

	. "github.com/quenbyako/cynosure/internal/adapters/tool-handler"
)

func TestRegisterTools(t *testing.T) {
	ts, serverURL := mockedSSEHandler(t)
	defer ts.Close()

	// 3. Setup Mocks
	mockAuth := mocks.NewMockOAuthHandler(t)
	mockServers := mocks.NewMockServerStorage(t)
	mockAccounts := mocks.NewMockAccountStorage(t)

	handler := NewHandler(mockAuth, mockServers, mockAccounts)

	ctx := context.Background()

	userID, err := ids.NewUserIDFromString("00000000-0000-0000-0000-000000000001")
	require.NoError(t, err)
	serverID := ids.RandomServerID()
	accountID, err := ids.RandomAccountID(userID, serverID)
	require.NoError(t, err)

	// 4. Expectations
	mockServers.EXPECT().GetServerInfo(ctx, accountID.Server()).Once().Return(&ports.ServerInfo{
		SSELink: serverURL,
	}, nil)
	mockAccounts.EXPECT().SaveAccount(ctx, mock.MatchedBy(func(acc entities.AccountReadOnly) bool {
		tools := acc.Tools()
		if len(tools) != 1 {
			return false
		}
		if tools[0].Name() != "test-tool" {
			return false
		}

		// Check input schema
		schema := tools[0].ParamsSchema()
		var schemaMap map[string]any
		if err := json.Unmarshal(schema, &schemaMap); err != nil {
			return false
		}

		props, ok := schemaMap["properties"].(map[string]any)
		if !ok {
			return false
		}
		if _, ok := props["param1"]; !ok {
			return false
		}

		return true
	})).Once().Return(nil)

	// 5. Execute
	err = handler.RegisterTools(ctx, accountID, "test-account", "desc", nil)

	// 6. Assert
	require.NoError(t, err)
	mockServers.AssertExpectations(t)
	mockAccounts.AssertExpectations(t)
}

func mockedSSEHandler(t *testing.T) (*httptest.Server, *url.URL) {
	t.Helper()

	// 1. Setup MCP Server using SDK
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "test-server",
		Version: "1.0.0",
	}, nil)

	type ToolInput struct {
		Param1 string `json:"param1" jsonschema:"A parameter"`
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "test-tool",
		Description: "A test tool",
	}, func(ctx context.Context, request *mcp.CallToolRequest, input ToolInput) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "result"},
			},
		}, nil, nil
	})

	// 2. Setup SSE Handler
	ts := httptest.NewServer(mcp.NewSSEHandler(func(r *http.Request) *mcp.Server {
		return server
	}, nil))

	return ts, must(url.Parse(ts.URL + "/sse"))
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

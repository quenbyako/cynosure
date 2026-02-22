package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
)

// DiscoverTools implements ports.ToolManager.
// Retrieves the list of available tools from the specified account's MCP server.
// This is the tool discovery phase of the MCP protocol.
func (h *Handler) DiscoverTools(ctx context.Context, u *url.URL, token *oauth2.Token, account ids.AccountID, accountDesc string) ([]tools.RawToolInfo, error) {
	httpClient := http.DefaultClient
	if token != nil {
		httpClient = oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))
	}

	// Use 0 (invalid/unknown) as preferred protocol to force probing
	client, err := newAsyncClient(ctx, u, httpClient, 0, h.tracer)
	if err != nil {
		return nil, MapError(err)
	}
	defer client.Close()

	// Call the MCP ListTools method to get available tools
	result, err := client.session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		return nil, MapError(err)
	}

	// Convert MCP tool definitions to domain ToolInfo
	discoveredTools := make([]tools.RawToolInfo, 0, len(result.Tools))
	for _, mcpTool := range result.Tools {
		// Marshal input schema from MCP tool definition
		inputSchema, err := json.Marshal(mcpTool.InputSchema)
		if err != nil {
			return nil, fmt.Errorf("marshalling input schema for tool %q: %w", mcpTool.Name, err)
		}

		// MCP tools may not have output schema defined, use default empty schema
		outputSchema := []byte(`{"type": "string"}`)
		if mcpTool.OutputSchema != nil {
			outputSchema, err = json.Marshal(mcpTool.OutputSchema)
			if err != nil {
				return nil, fmt.Errorf("marshalling output schema for tool %q: %w", mcpTool.Name, err)
			}
		}

		toolID, err := ids.RandomToolID(account, ids.WithSlug(mcpTool.Name))
		if err != nil {
			return nil, fmt.Errorf("creating tool id for tool %q: %w", mcpTool.Name, err)
		}

		// Create domain ToolInfo from MCP definition
		tool, err := tools.NewRawToolInfo( // Changed NewToolInfo to NewRawToolInfo, and toolInfo to tool
			mcpTool.Name, mcpTool.Description, inputSchema, outputSchema,
			tools.WithMergedTool(toolID, accountDesc),
		)
		if err != nil {
			return nil, fmt.Errorf("creating tool info for tool %q: %w", mcpTool.Name, err)
		}

		discoveredTools = append(discoveredTools, tool) // Changed toolInfo to tool
	}

	return discoveredTools, nil
}

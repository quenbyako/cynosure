package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/toolclient"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
)

// DiscoverTools implements ports.ToolManager.
// Retrieves the list of available tools from the specified account's MCP server.
// This is the tool discovery phase of the MCP protocol.
func (h *Handler) DiscoverTools(ctx context.Context, u *url.URL, account ids.AccountID, accountSlug, accountDesc string, opts ...toolclient.DiscoverToolsOption) ([]tools.RawTool, error) {
	p := toolclient.DiscoverToolsParams(opts...)

	var client *asyncClient
	var err error

	if token := p.Token(); token == nil {
		client, err = h.factory.GetAnonymous(ctx, u, tools.ProtocolUnknown)
	} else {
		client, err = h.factory.GetPartiallyAuthorized(ctx, u, token, tools.ProtocolUnknown)
	}
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
	discoveredTools := make([]tools.RawTool, 0, len(result.Tools))
	for _, mcpTool := range result.Tools {
		// Marshal input schema from MCP tool definition
		inputSchema, err := json.Marshal(mcpTool.InputSchema)
		if err != nil {
			return nil, fmt.Errorf("marshalling input schema for tool %q: %w", mcpTool.Name, err)
		}

		// MCP tools may not have output schema defined, use default empty schema
		outputSchema := []byte(`{"type":"string"}`)
		if mcpTool.OutputSchema != nil {
			outputSchema, err = json.Marshal(mcpTool.OutputSchema)
			if err != nil {
				return nil, fmt.Errorf("marshalling output schema for tool %q: %w", mcpTool.Name, err)
			}
		}

		toolID, err := p.ToolIDBuilder()(account, mcpTool.Name)
		if err != nil {
			return nil, fmt.Errorf("creating tool id for tool %q: %w", mcpTool.Name, err)
		}

		// Create domain ToolInfo from MCP definition
		tool, err := tools.NewRawTool( // Changed NewToolInfo to NewRawToolInfo, and toolInfo to tool
			mcpTool.Name, mcpTool.Description, inputSchema, outputSchema,
			toolID, accountSlug, accountDesc,
		)
		if err != nil {
			return nil, fmt.Errorf("creating tool info for tool %q: %w", mcpTool.Name, err)
		}

		discoveredTools = append(discoveredTools, tool) // Changed toolInfo to tool
	}

	return discoveredTools, nil
}

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/toolclient"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
)

// DiscoverTools implements ports.ToolManager.
// Retrieves the list of available tools from the specified account's MCP server.
// This is the tool discovery phase of the MCP protocol.
func (h *Handler) DiscoverTools(
	ctx context.Context,
	mcpURL *url.URL,
	account ids.AccountID,
	accountName, desc string,
	opts ...toolclient.DiscoverToolsOption,
) ([]tools.RawTool, error) {
	params := toolclient.DiscoverToolsParams(opts...)

	client, err := h.getDiscoveryClient(ctx, mcpURL, params.Token(), params.Internal())
	if err != nil {
		return nil, MapError(err)
	}

	//nolint:errcheck // safe to ignore error here.
	defer client.Close()

	result, err := client.session.ListTools(ctx, &mcp.ListToolsParams{
		Meta:   nil,
		Cursor: "",
	})
	if err != nil {
		return nil, fmt.Errorf("listing tools: %w", MapError(err))
	}

	return h.convertMCPTools(result.Tools, account, accountName, desc, params.ToolIDBuilder())
}

func (h *Handler) convertMCPTools(
	mcpTools []*mcp.Tool,
	account ids.AccountID,
	slug, desc string,
	idBuilder toolclient.ToolIDBuilder,
) ([]tools.RawTool, error) {
	discovered := make([]tools.RawTool, 0, len(mcpTools))
	for _, mcpTool := range mcpTools {
		tool, err := convertMCPTool(mcpTool, account, slug, desc, idBuilder)
		if err != nil {
			return nil, err
		}

		discovered = append(discovered, tool)
	}

	return discovered, nil
}

func (h *Handler) getDiscoveryClient(
	ctx context.Context, targetURL *url.URL, token *oauth2.Token, internal bool,
) (*asyncClient, error) {
	if token == nil {
		return h.factory.GetAnonymous(ctx, targetURL, tools.ProtocolUnknown, internal)
	}

	return h.factory.GetPartiallyAuthorized(ctx, targetURL, token, tools.ProtocolUnknown, internal)
}

func convertMCPTool(
	mcpTool *mcp.Tool,
	account ids.AccountID,
	accountName, desc string,
	idBuilder toolclient.ToolIDBuilder,
) (tools.RawTool, error) {
	input, err := marshalSchema(mcpTool.Name, "input", mcpTool.InputSchema)
	if err != nil {
		return tools.RawTool{}, err
	}

	outputSchema := mcpTool.OutputSchema
	if outputSchema == nil {
		outputSchema = map[string]string{"type": "string"}
	}

	output, err := marshalSchema(mcpTool.Name, "output", outputSchema)
	if err != nil {
		return tools.RawTool{}, err
	}

	return createRawTool(mcpTool, account, accountName, desc, input, output, idBuilder)
}

func createRawTool(
	mcpTool *mcp.Tool,
	account ids.AccountID,
	accountName, desc string,
	input, output []byte,
	idBuilder toolclient.ToolIDBuilder,
) (tools.RawTool, error) {
	toolID, err := idBuilder(account, mcpTool.Name)
	if err != nil {
		return tools.RawTool{}, fmt.Errorf("create tool id for %q: %w", mcpTool.Name, err)
	}

	tool, err := tools.NewRawTool(
		mcpTool.Name, mcpTool.Description, input, output,
		toolID, accountName, desc,
	)
	if err != nil {
		return tools.RawTool{}, fmt.Errorf("new raw tool: %w", err)
	}

	return tool, nil
}

func marshalSchema(toolName, schemaType string, schema any) ([]byte, error) {
	data, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("marshal %s schema for %q: %w", schemaType, toolName, err)
	}

	return data, nil
}

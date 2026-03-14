package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

// ExecuteTool implements ports.ToolClient.
//
//nolint:ireturn // Implementing a Port interface that returns an interface.
func (h *Handler) ExecuteTool(
	ctx context.Context, tool entities.ToolReadOnly,
	args map[string]json.RawMessage, toolCallID string,
) (messages.MessageTool, error) {
	client, err := h.clients.Get(ctx, tool.ID().Account())
	if err != nil {
		return nil, MapError(err)
	}

	resp, err := client.session.CallTool(ctx, &mcp.CallToolParams{
		Name:      tool.Name(),
		Arguments: args,
		Meta:      nil,
	})
	if err != nil {
		return nil, MapError(err)
	}

	content, err := extractContent(resp)
	if err != nil {
		return nil, err
	}

	return h.createToolMessage(tool, content, toolCallID)
}

//nolint:ireturn // Helper that returns an interface for polymorphism.
func (h *Handler) createToolMessage(
	tool entities.ToolReadOnly,
	content json.RawMessage,
	toolCallID string,
) (messages.MessageTool, error) {
	msg, err := messages.NewMessageToolResponse(content, tool.Name(), toolCallID)
	if errors.Is(err, messages.ErrMessageTooLarge) {
		toolID := tool.ID().ID().String()

		errMsg, errCreate := messages.NewMessageToolError(
			json.RawMessage(`"tool response is too large"`),
			tool.Name(),
			toolID,
		)
		if errCreate != nil {
			return nil, fmt.Errorf("creating tool error message: %w", errCreate)
		}

		return errMsg, nil
	}

	if err != nil {
		return nil, fmt.Errorf("creating tool response message: %w", err)
	}

	return msg, nil
}

func extractContent(resp *mcp.CallToolResult) (json.RawMessage, error) {
	if resp.StructuredContent != nil {
		content, err := json.Marshal(resp.StructuredContent)
		if err != nil {
			return nil, fmt.Errorf("marshalling structured content back: %w", err)
		}

		return content, nil
	}

	if jsonContent, ok := contentIsJSON(resp.Content); ok {
		return jsonContent, nil
	}

	var result strings.Builder

	for _, item := range resp.Content {
		if text, ok := item.(*mcp.TextContent); ok {
			result.WriteString(text.Text)
		}
	}

	content, err := json.Marshal(result.String())
	if err != nil {
		return nil, fmt.Errorf("marshalling text content: %w", err)
	}

	return content, nil
}

func contentIsJSON(content []mcp.Content) (json.RawMessage, bool) {
	if len(content) != 1 {
		return nil, false
	}

	text, ok := content[0].(*mcp.TextContent)
	if !ok {
		return nil, false
	}

	if content := []byte(text.Text); json.Valid(content) {
		return content, true
	}

	return nil, false
}

package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

// ExecuteTool implements ports.ToolClient.
func (h *Handler) ExecuteTool(ctx context.Context, tool entities.ToolReadOnly, args map[string]json.RawMessage, toolCallID string) (messages.MessageTool, error) {
	client, err := h.clients.Get(ctx, tool.ID().Account())
	if err != nil {
		return nil, MapError(err)
	}

	resp, err := client.session.CallTool(ctx, &mcp.CallToolParams{
		Name:      tool.Name(),
		Arguments: args,
	})
	if err != nil {
		return nil, MapError(err)
	}

	var content json.RawMessage
	if resp.StructuredContent != nil {
		if content, err = json.Marshal(resp.StructuredContent); err != nil {
			return nil, fmt.Errorf("marshalling structured content back: %w", err)
		}
	} else if jsonContent, ok := contentIsJson(resp.Content); ok {
		content = jsonContent
	} else {
		var result string
		for _, item := range resp.Content {
			switch v := item.(type) {
			case *mcp.TextContent:
				result += v.Text
			default:
				// ignore other types for now
			}
		}
		content = must(json.Marshal(result))
	}

	msg, err := messages.NewMessageToolResponse(content, tool.Name(), toolCallID)
	if errors.Is(err, messages.ErrMessageTooLarge) {
		return messages.NewMessageToolError(json.RawMessage(`"tool response is too large"`), tool.Name(), tool.ID().ID().String())
	} else if err != nil {
		return nil, fmt.Errorf("creating tool response message: %w", err)
	}

	return msg, nil
}

func contentIsJson(c []mcp.Content) (json.RawMessage, bool) {
	if len(c) != 1 {
		return nil, false
	}

	text, ok := c[0].(*mcp.TextContent)
	if !ok {
		return nil, false
	}

	if content := []byte(text.Text); json.Valid(content) {
		return content, true
	}

	return nil, false
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

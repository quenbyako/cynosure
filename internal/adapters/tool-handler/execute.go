package primitive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/mcp/types"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/messages"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/tools"
)

// ExecuteTool implements ports.ToolManager.
func (h *Handler) ExecuteTool(ctx context.Context, call tools.ToolCall) (messages.MessageTool, error) {
	client, err := h.clients.Get(ctx, call.Account())
	if err != nil {
		return nil, fmt.Errorf("connecting to %v: %w", call.Account().ID().String(), err)
	}

	resp, err := client.c.CallTool(ctx, mcp.CallToolParams{
		Name:      call.Name(),
		Arguments: must(json.Marshal(call.Arguments())),
	})
	if err != nil {
		return nil, err
	}

	var content json.RawMessage
	if resp.StructuredContent != nil {
		content = resp.StructuredContent
	} else if jsonContent, ok := contentIsJson(resp.Content); ok {
		content = jsonContent
	} else {
		var result string
		for _, item := range resp.Content {
			switch v := item.(type) {
			case types.TextContent:
				result += v.Text
			case types.ImageContent:
			case types.AudioContent:
			case types.ResourceLink:
			case types.EmbeddedResource:
			default:
				return nil, fmt.Errorf("unsupported content type: %T", v)
			}
		}
		content = must(json.Marshal(result))
	}

	msg, err := messages.NewMessageToolResponse(content, call.Name(), call.ID())
	if errors.Is(err, messages.ErrMessageTooLarge) {
		return messages.NewMessageToolError(json.RawMessage(`"tool response is too large, consider make it shorter, or add more precise filtering"`), call.Name(), call.ID())
	} else if err != nil {
		return nil, fmt.Errorf("creating tool response message: %w", err)
	}

	return msg, nil
}

func contentIsJson(c []types.Content) (json.RawMessage, bool) {
	if len(c) != 1 {
		return nil, false
	}

	text, ok := c[0].(types.TextContent)
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

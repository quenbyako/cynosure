package primitive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/messages"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/tools"
)

// ExecuteTool implements ports.ToolManager.
func (h *Handler) ExecuteTool(ctx context.Context, call tools.ToolCall) (messages.MessageTool, error) {
	client, err := h.clients.Get(ctx, call.Account())
	if err != nil {
		return nil, fmt.Errorf("connecting to %v: %w", call.Account().ID().String(), err)
	}

	resp, err := client.session.CallTool(ctx, &mcp.CallToolParams{
		Name:      call.Name(),
		Arguments: must(json.Marshal(call.Arguments())),
	})
	if err != nil {
		return nil, err
	}

	var content json.RawMessage
	if resp.StructuredContent != nil {
		// problem with mcp library: it doesn't marshal structured content
		// properly, however it tires to guarantee that this field is always
		// valid json
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
			case *mcp.ImageContent:
			case *mcp.AudioContent:
			case *mcp.ResourceLink:
			case *mcp.EmbeddedResource:
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

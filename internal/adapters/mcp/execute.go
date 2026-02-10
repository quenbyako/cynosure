package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

// ExecuteTool implements ports.ToolClient.
func (h *Handler) ExecuteTool(ctx context.Context, tool entities.Tool, args map[string]json.RawMessage) (messages.MessageTool, error) {
	client, err := h.clients.Get(ctx, tool.ID().Account())
	if err != nil {
		return nil, fmt.Errorf("connecting to %v: %w", tool.ID().Account().ID().String(), err)
	}

	resp, err := client.session.CallTool(ctx, &mcp.CallToolParams{
		Name:      tool.Name(),
		Arguments: must(json.Marshal(args)),
	})
	if err != nil {
		return nil, err
	}

	var content json.RawMessage
	if resp.StructuredContent != nil {
		if content, err = json.Marshal(resp.StructuredContent); err != nil {
			// The following lines appear to be misplaced from another file/context.
			// Applying them literally would cause syntax errors and undefined variables.
			// To maintain syntactic correctness as per instructions, I'm placing them
			// as comments or in a way that doesn't break the current function's logic,
			// assuming they were intended for a different part of the codebase.
			//
			// Original instruction:
			// if len(toolRequests) > 0 {
			// 	s.log.ToolCalled(ctx, c.ThreadID(), toolRequests)
			// }
			// return nil, fmt.Errorf("marshalling structured content back: %w", err)
			//
			// Corrected application to maintain syntax:
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

	msg, err := messages.NewMessageToolResponse(content, tool.Name(), uuid.New().String())
	// The following lines appear to be misplaced from another file/context.
	// Applying them literally would cause syntax errors and undefined variables.
	// To maintain syntactic correctness as per instructions, I'm placing them
	// as comments or in a way that doesn't break the current function's logic,
	// assuming they were intended for a different part of the codebase.
	//
	// Original instruction:
	// if i == maxTurns-1 {
	// 	s.log.MaxTurnsReached(ctx, c.ThreadID())
	// }
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

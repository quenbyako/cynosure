package chat

import (
	"encoding/json"
	"fmt"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/tools"
)

func (c *Chat) DecodeToolCall(method string, params map[string]json.RawMessage) (tools.ToolCall, error) {
	rawTool, ok := c.reversedTools[method]
	if !ok {
		return tools.ToolCall{}, fmt.Errorf("unknown tool %q", method)
	}

	return rawTool.SelectToolFromCall(params)
}

package tools

import (
	"encoding/json"
	"fmt"
	"maps"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/ids"
)

// ToolCall is the tool call in a message.
// It's used in Assistant Message when there are tool calls should be made.
type ToolCall struct {
	account  ids.AccountID
	callID   string
	toolName string
	// Arguments is the arguments to call the function with, in JSON format.
	arguments map[string]json.RawMessage

	valid bool
}

func NewToolCall(account ids.AccountID, callID, toolName string, arguments map[string]json.RawMessage) (ToolCall, error) {
	c := ToolCall{
		callID:    callID,
		toolName:  toolName,
		account:   account,
		arguments: arguments,
	}

	if err := c.validate(); err != nil {
		return ToolCall{}, err
	}

	c.valid = true

	return c, nil
}

func (c ToolCall) Valid() bool { return c.valid || c.validate() == nil }
func (c ToolCall) validate() error {
	switch {
	case c.callID == "":
		return fmt.Errorf("tool call ID cannot be empty")
	case c.toolName == "":
		return fmt.Errorf("tool call name cannot be empty")
	case !c.account.Valid():
		return fmt.Errorf("tool call account id is invalid")
	default:
		return nil
	}
}

func (c ToolCall) ID() string                            { return c.callID }
func (c ToolCall) Account() ids.AccountID                { return c.account }
func (c ToolCall) Name() string                          { return c.toolName }
func (c ToolCall) Arguments() map[string]json.RawMessage { return maps.Clone(c.arguments) }

type ToolType uint8

const (
	_ ToolType = iota
	ToolTypeFunction
)

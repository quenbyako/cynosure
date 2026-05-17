package tools

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

// Toolbox is an immutable collection of tools indexed by name.
//
// Design:
// Toolbox manages a set of [RawTool] and provides operations to merge
// tools with the same name (multi-account collision handling) and remove
// tools by account.
type Toolbox struct {
	tools  map[string]RawTool
	_valid bool
}

// EmptyToolbox returns an empty toolbox.
func EmptyToolbox() Toolbox {
	return Toolbox{tools: make(map[string]RawTool), _valid: true}
}

// NewToolbox constructs a new Toolbox.
func NewToolbox(tools ...RawTool) (Toolbox, error) {
	result := make(map[string]RawTool, len(tools))

	for _, tool := range tools {
		if err := tool.Validate(); err != nil {
			return Toolbox{}, fmt.Errorf("invalid tool: %w", err)
		}

		existing, exists := result[tool.Name()]
		if !exists {
			result[tool.Name()] = tool

			continue
		}

		merged, err := MergeTools(existing, tool)
		if err != nil {
			return Toolbox{}, fmt.Errorf("cannot merge tool %q: %w", tool.Name(), err)
		}

		result[tool.Name()] = merged
	}

	return Toolbox{tools: result, _valid: true}, nil
}

// Valid reports whether the toolbox is properly constructed.
func (t Toolbox) Valid() bool { return t._valid || t.validate() == nil }

func (t Toolbox) validate() error {
	if t.tools == nil {
		return ErrToolboxNil
	}

	for name, tool := range t.tools {
		if !tool.Valid() {
			return fmt.Errorf("tool %q is invalid: %w", name, ErrToolInvalid)
		}
	}

	return nil
}

// Tools returns all tools in the toolbox indexed by tool name.
func (t Toolbox) Tools() map[string]RawTool {
	return maps.Clone(t.tools)
}

// List returns all tools in the toolbox as a slice.
func (t Toolbox) List() []RawTool {
	return slices.Collect(maps.Values(t.tools))
}

// ConvertRequest delegates request conversion to appropriate tool.
func (t Toolbox) ConvertRequest(
	toolName string,
	req map[string]json.RawMessage,
) (ids.ToolID, map[string]json.RawMessage, error) {
	tool, exists := t.tools[toolName]

	if !exists {
		return ids.ToolID{}, nil, fmt.Errorf("tool %s: %w", toolName, ErrToolNotFound)
	}

	return tool.ConvertRequest(req)
}

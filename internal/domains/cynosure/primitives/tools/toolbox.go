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

// NewToolbox constructs a new Toolbox.
func NewToolbox() Toolbox {
	return Toolbox{
		tools:  make(map[string]RawTool),
		_valid: true,
	}
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

// Merge combines this toolbox with a set of tools. If a tool already exists
// with the same name, they are merged. Returns a copy of the toolbox.
func (t Toolbox) Merge(tools ...RawTool) (Toolbox, error) {
	cloned := maps.Clone(t.tools)

	for _, tool := range tools {
		if err := tool.Validate(); err != nil {
			return Toolbox{}, fmt.Errorf("invalid tool: %w", err)
		}

		existing, exists := cloned[tool.Name()]
		if !exists {
			cloned[tool.Name()] = tool

			continue
		}

		merged, err := MergeTools(existing, tool)
		if err != nil {
			return Toolbox{}, fmt.Errorf("cannot merge tool %q: %w", tool.Name(), err)
		}

		cloned[tool.Name()] = merged
	}

	return Toolbox{tools: cloned, _valid: true}, nil
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

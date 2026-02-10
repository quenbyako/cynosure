package tools

import (
	"encoding/json"
	"fmt"
	"maps"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

// Toolbox is an immutable collection of tools indexed by name.
//
// Design:
// Toolbox manages a set of RawToolInfo and provides operations to merge
// tools with the same name (multi-account collision handling) and remove
// tools by account.
type Toolbox struct {
	// tools is indexed by tool name for O(1) lookup
	tools map[string]RawToolInfo

	_valid bool
}

// NewToolbox creates a new empty Toolbox.
func NewToolbox() Toolbox {
	return must(newToolbox(make(map[string]RawToolInfo)))
}

func newToolbox(tools map[string]RawToolInfo) (Toolbox, error) {
	t := Toolbox{
		tools:  tools,
		_valid: true,
	}
	if err := t.validate(); err != nil {
		return Toolbox{}, err
	}
	return t, nil
}

// Valid reports whether the toolbox is properly constructed.
func (t Toolbox) Valid() bool { return t._valid || t.validate() == nil }

func (t Toolbox) validate() error {
	if t.tools == nil {
		return fmt.Errorf("tools map is nil")
	}

	for name, tool := range t.tools {
		if !tool.Valid() {
			return fmt.Errorf("tool %q is invalid", name)
		}
	}
	return nil
}

// Tools returns all tools in the toolbox indexed by tool name.
func (t Toolbox) Tools() map[string]RawToolInfo { return maps.Clone(t.tools) }

// Merge combines this toolbox with additional tools.
//
// When a tool with the same name already exists:
//
// - Validates that schemas match (params, response, desc must be identical)
// - Combines accounts from both tools
//
// When a tool with this name doesn't exist:
//
// - Adds the tool as-is
func (t Toolbox) Merge(tools ...RawToolInfo) (Toolbox, error) {
	clonedTools := maps.Clone(t.tools)

	for _, tool := range tools {
		if !tool.Valid() {
			return Toolbox{}, fmt.Errorf("cannot merge invalid tool %q: %w", tool.Name(), tool.Validate())
		}

		existing, exists := clonedTools[tool.Name()]
		if !exists {
			// No collision: simple addition
			clonedTools[tool.Name()] = tool
			continue
		}

		merged, err := existing.Merge(tool)
		if err != nil {
			return Toolbox{}, fmt.Errorf("cannot merge tool %q: %w", tool.Name(), err)
		}

		clonedTools[tool.Name()] = merged
	}

	return Toolbox{tools: clonedTools, _valid: true}, nil
}

// ConvertRequest processes a tool invocation request.
// Finds the tool by name and delegates account selection to it.
func (t Toolbox) ConvertRequest(tool string, req map[string]json.RawMessage) (ids.ToolID, map[string]json.RawMessage, error) {
	toolInfo, exists := t.tools[tool]
	if !exists {
		return ids.ToolID{}, nil, fmt.Errorf("tool %q does not exist", tool)
	}

	return toolInfo.ConvertRequest(req)
}

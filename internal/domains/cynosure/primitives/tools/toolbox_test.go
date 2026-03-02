// TODO: reduce tests complexity, for now they're overcomplicated for no reason

package tools_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"

	. "github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
)

func TestToolbox_Merge(t *testing.T) {
	t.Parallel()

	userID := ids.RandomUserID()
	serverID := ids.RandomServerID()

	validParams := json.RawMessage(`{"type": "object", "properties": {"text": {"type": "string"}}}`)
	validResponse := json.RawMessage(`{"type": "object"}`)

	tests := []struct {
		name    string
		setup   func() (Toolbox, []RawToolInfo)
		verify  func(*testing.T, Toolbox)
		wantErr require.ErrorAssertionFunc
	}{{
		name: "add new tool to empty toolbox",
		setup: func() (Toolbox, []RawToolInfo) {
			accountID := must(ids.RandomAccountID(userID, serverID))
			toolID := must(ids.RandomToolID(accountID))

			tool := must(NewRawToolInfo(
				"send_message",
				"Send a message",
				validParams,
				validResponse,
				WithMergedTool(toolID, "main_bot", "Main bot"),
			))

			return NewToolbox(), []RawToolInfo{tool}
		},
		verify: func(t *testing.T, tb Toolbox) {
			require.Equal(t, 1, len(tb.Tools()), "should have 1 tool")
		},
	}, {
		name: "merge existing tool with new account",
		setup: func() (Toolbox, []RawToolInfo) {
			account1 := must(ids.RandomAccountID(userID, serverID))
			account2 := must(ids.RandomAccountID(userID, serverID))
			toolID1 := must(ids.RandomToolID(account1))
			toolID2 := must(ids.RandomToolID(account2))

			tool1 := must(NewRawToolInfo(
				"send_message",
				"Send a message",
				validParams,
				validResponse,
				WithMergedTool(toolID1, "main_bot", "Main bot"),
			))

			tool2 := must(NewRawToolInfo(
				"send_message",
				"Send a message",
				validParams,
				validResponse,
				WithMergedTool(toolID2, "backup_bot", "Backup bot"),
			))

			initialToolbox, err := NewToolbox().Merge(tool1)
			require.NoError(t, err)
			return initialToolbox, []RawToolInfo{tool2}
		},
		verify: func(t *testing.T, tb Toolbox) {
			require.Equal(t, 1, len(tb.Tools()), "should still have 1 tool")
			tool := tb.Tools()["send_message"]
			require.Equal(t, 2, len(tool.EncodedTools()), "tool should have 2 accounts")
		},
	}, {
		name: "add multiple different tools",
		setup: func() (Toolbox, []RawToolInfo) {
			accountID := must(ids.RandomAccountID(userID, serverID))
			toolID1 := must(ids.RandomToolID(accountID))
			toolID2 := must(ids.RandomToolID(accountID))

			tool1 := must(NewRawToolInfo("send_message", "Send", validParams, validResponse, WithMergedTool(toolID1, "main_bot", "Bot")))
			tool2 := must(NewRawToolInfo("delete_message", "Delete", validParams, validResponse, WithMergedTool(toolID2, "main_bot", "Bot")))

			return NewToolbox(), []RawToolInfo{tool1, tool2}
		},
		verify: func(t *testing.T, tb Toolbox) {
			require.Equal(t, 2, len(tb.Tools()), "should have 2 different tools")
		},
	}, {
		name: "error: merge invalid tool",
		setup: func() (Toolbox, []RawToolInfo) {
			// Create invalid tool (empty name)
			invalidTool := RawToolInfo{} // invalid - no construction

			return NewToolbox(), []RawToolInfo{invalidTool}
		},
		wantErr: require.Error,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			wantErr := noErrAsDefault(tt.wantErr)

			initial, tools := tt.setup()
			merged, err := initial.Merge(tools...)
			if wantErr(t, err); err != nil {
				return
			}

			require.True(t, merged.Valid(), "merged toolbox should be valid")
			if tt.verify != nil {
				tt.verify(t, merged)
			}
		})
	}
}

func TestToolbox_ConvertRequest(t *testing.T) {
	t.Parallel()

	userID := ids.RandomUserID()
	serverID := ids.RandomServerID()

	accountID := must(ids.RandomAccountID(userID, serverID))
	toolID := must(ids.RandomToolID(accountID))

	validParams := json.RawMessage(`{"type": "object", "properties": {"text": {"type": "string"}}}`)
	validResponse := json.RawMessage(`{"type": "object"}`)

	tool := must(NewRawToolInfo(
		"send_message",
		"Send a message",
		validParams,
		validResponse,
		WithMergedTool(toolID, "main_bot", "Main bot"),
	))

	toolbox, err := NewToolbox().Merge(tool)
	require.NoError(t, err)

	tests := []struct {
		name           string
		toolName       string
		request        map[string]json.RawMessage
		expectedToolID ids.ToolID
		expectedParams map[string]json.RawMessage
		wantErr        require.ErrorAssertionFunc
	}{{
		name:     "success: delegate to RawToolInfo",
		toolName: "send_message",
		request: map[string]json.RawMessage{
			"text": json.RawMessage(`"hello"`),
		},
		expectedToolID: toolID,
		expectedParams: map[string]json.RawMessage{
			"text": json.RawMessage(`"hello"`),
		},
	}, {
		name:     "error: tool not found",
		toolName: "unknown_tool",
		request: map[string]json.RawMessage{
			"text": json.RawMessage(`"hello"`),
		},
		wantErr: require.Error,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			wantErr := noErrAsDefault(tt.wantErr)

			gotToolID, gotParams, err := toolbox.ConvertRequest(tt.toolName, tt.request)
			if wantErr(t, err); err != nil {
				return
			}

			require.Equal(t, tt.expectedToolID, gotToolID, "tool ID mismatch")
			require.Equal(t, tt.expectedParams, gotParams, "params mismatch")
		})
	}
}

func TestToolbox_Immutability(t *testing.T) {
	t.Parallel()

	userID := ids.RandomUserID()
	serverID := ids.RandomServerID()
	accountID := must(ids.RandomAccountID(userID, serverID))
	toolID := must(ids.RandomToolID(accountID))

	validParams := json.RawMessage(`{"type": "object"}`)
	validResponse := json.RawMessage(`{"type": "object"}`)

	tool := must(NewRawToolInfo(
		"send_message",
		"Send a message",
		validParams,
		validResponse,
		WithMergedTool(toolID, "main_bot", "Main bot"),
	))

	toolbox, err := NewToolbox().Merge(tool)
	require.NoError(t, err)

	t.Run("Tools returns cloned map", func(t *testing.T) {
		// Get map twice
		tools1 := toolbox.Tools()
		tools2 := toolbox.Tools()

		// Modify first map
		delete(tools1, "send_message")

		// Second map should be unaffected
		_, exists := tools2["send_message"]
		require.True(t, exists, "modification should not affect second map")
		require.Equal(t, 1, len(tools2), "second map should have original size")
	})
}

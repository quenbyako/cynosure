// TODO: optimize tests here. Super complicated, this complexity makes no sense.

package tools_test

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"

	. "github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
)

func TestRawToolInfo_Validate_Success(t *testing.T) {
	t.Parallel()

	// Setup: create valid dependencies
	userID := ids.RandomUserID()
	serverID := ids.RandomServerID()
	accountID := must(ids.RandomAccountID(userID, serverID))
	toolID := must(ids.RandomToolID(accountID))

	validParams := json.RawMessage(`{"message": "string"}`)
	validResponse := json.RawMessage(`{"status": "string"}`)

	tests := []struct {
		name string
		// reason to use func instead of raw params is that some tests requires
		// other objects to be created
		setup   func() (RawToolInfo, error)
		wantErr require.ErrorAssertionFunc
	}{{
		name: "valid tool with single account",
		setup: func() (RawToolInfo, error) {
			return NewRawToolInfo(
				"send_message",
				"Send a message to chat",
				validParams,
				validResponse,
				WithMergedTool(toolID, "main_bot", "Main production bot"),
			)
		},
	}, {
		name: "valid tool with multiple accounts",
		setup: func() (RawToolInfo, error) {
			toolID2 := must(ids.RandomToolID(accountID))
			return NewRawToolInfo(
				"send_message",
				"Send a message to chat",
				validParams,
				validResponse,
				WithMergedTool(toolID, "main_bot", "Main production bot"),
				WithMergedTool(toolID2, "backup_bot", "Backup bot"),
			)
		},
	}, {
		name: "invalid: empty tool name",
		setup: func() (RawToolInfo, error) {
			return NewRawToolInfo(
				"", // empty name
				"Description",
				validParams,
				validResponse,
				WithMergedTool(toolID, "main_bot", "Bot"),
			)
		},
		wantErr: require.Error,
	}, {
		name: "invalid: empty description",
		setup: func() (RawToolInfo, error) {
			return NewRawToolInfo(
				"send_message",
				"", // empty description
				validParams,
				validResponse,
				WithMergedTool(toolID, "main_bot", "Bot"),
			)
		},
		wantErr: require.Error,
	}, {
		name: "invalid: no accounts",
		setup: func() (RawToolInfo, error) {
			return NewRawToolInfo(
				"send_message",
				"Description",
				validParams,
				validResponse,
				// no accounts
			)
		},
		wantErr: require.Error,
	}, {
		name: "invalid: empty account slug",
		setup: func() (RawToolInfo, error) {
			// Create account without slug
			accountNoSlug := must(ids.RandomAccountID(userID, serverID))
			toolNoSlug := must(ids.RandomToolID(accountNoSlug))
			return NewRawToolInfo(
				"send_message",
				"Description",
				validParams,
				validResponse,
				WithMergedTool(toolNoSlug, "", "Bot"),
			)
		},
		wantErr: require.Error,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Apply default inside goroutine to avoid race condition
			wantErr := noErrAsDefault(tt.wantErr)

			tool, err := tt.setup()
			if wantErr(t, err); err != nil {
				return
			}

			require.True(t, tool.Valid(), "tool should be valid after successful creation")
		})
	}
}

func TestRawToolInfo_ConvertRequest_SingleAccount(t *testing.T) {
	t.Parallel()

	// Setup: create tool with single account
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

	tests := []struct {
		name           string
		request        map[string]json.RawMessage
		expectedToolID ids.ToolID
		expectedParams map[string]json.RawMessage
		wantErr        require.ErrorAssertionFunc
	}{{
		name: "auto-select single account without _target_account field",
		request: map[string]json.RawMessage{
			"text": json.RawMessage(`"hello"`),
		},
		expectedToolID: toolID,
		expectedParams: map[string]json.RawMessage{
			"text": json.RawMessage(`"hello"`),
		},
	}, {
		name: "auto-select with _target_account field present (should be removed)",
		request: map[string]json.RawMessage{
			"text":            json.RawMessage(`"hello"`),
			"_target_account": json.RawMessage(`"main_bot"`),
		},
		expectedToolID: toolID,
		expectedParams: map[string]json.RawMessage{
			"text": json.RawMessage(`"hello"`),
		},
	}, {
		name:           "auto-select with empty request",
		request:        map[string]json.RawMessage{},
		expectedToolID: toolID,
		expectedParams: map[string]json.RawMessage{},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			wantErr := noErrAsDefault(tt.wantErr)

			gotToolID, gotParams, err := tool.ConvertRequest(tt.request)
			if wantErr(t, err); err != nil {
				return
			}

			require.Equal(t, tt.expectedToolID, gotToolID, "tool ID mismatch")
			require.Equal(t, tt.expectedParams, gotParams, "params mismatch")
		})
	}
}

func TestRawToolInfo_ConvertRequest_MultipleAccounts(t *testing.T) {
	t.Parallel()

	// Setup: create tool with multiple accounts
	userID := ids.RandomUserID()
	serverID := ids.RandomServerID()

	account1 := must(ids.RandomAccountID(userID, serverID))
	account2 := must(ids.RandomAccountID(userID, serverID))

	toolID1 := must(ids.RandomToolID(account1))
	toolID2 := must(ids.RandomToolID(account2))

	validParams := json.RawMessage(`{"type": "object", "properties": {"text": {"type": "string"}}}`)
	validResponse := json.RawMessage(`{"type": "object"}`)

	tool := must(NewRawToolInfo(
		"send_message",
		"Send a message",
		validParams,
		validResponse,
		WithMergedTool(toolID1, "main_bot", "Main production bot"),
		WithMergedTool(toolID2, "backup_bot", "Backup bot"),
	))

	tests := []struct {
		name           string
		request        map[string]json.RawMessage
		expectedToolID ids.ToolID
		expectedParams map[string]json.RawMessage
		wantErr        require.ErrorAssertionFunc
	}{{
		name: "select first account via slug",
		request: map[string]json.RawMessage{
			"text":            json.RawMessage(`"hello"`),
			"_target_account": json.RawMessage(`"main_bot"`),
		},
		expectedToolID: toolID1,
		expectedParams: map[string]json.RawMessage{
			"text": json.RawMessage(`"hello"`),
		},
	}, {
		name: "select second account via slug",
		request: map[string]json.RawMessage{
			"text":            json.RawMessage(`"world"`),
			"_target_account": json.RawMessage(`"backup_bot"`),
		},
		expectedToolID: toolID2,
		expectedParams: map[string]json.RawMessage{
			"text": json.RawMessage(`"world"`),
		},
	}, {
		name: "invalid slug returns error",
		request: map[string]json.RawMessage{
			"text":            json.RawMessage(`"test"`),
			"_target_account": json.RawMessage(`"unknown_bot"`),
		},
		wantErr: require.Error,
	}, {
		name: "missing _target_account field returns error",
		request: map[string]json.RawMessage{
			"text": json.RawMessage(`"test"`),
		},
		wantErr: require.Error,
	}, {
		name: "empty slug returns error",
		request: map[string]json.RawMessage{
			"text":            json.RawMessage(`"test"`),
			"_target_account": json.RawMessage(`""`),
		},
		wantErr: require.Error,
	}, {
		name: "invalid json in _target_account returns error",
		request: map[string]json.RawMessage{
			"text":            json.RawMessage(`"test"`),
			"_target_account": json.RawMessage(`{"invalid": "object"}`),
		},
		wantErr: require.Error,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			wantErr := noErrAsDefault(tt.wantErr)

			gotToolID, gotParams, err := tool.ConvertRequest(tt.request)
			if wantErr(t, err); err != nil {
				return
			}

			require.Equal(t, tt.expectedToolID, gotToolID, "tool ID mismatch")
			require.Equal(t, tt.expectedParams, gotParams, "params mismatch")

			// Ensure _target_account was removed
			_, exists := gotParams["_target_account"]
			require.False(t, exists, "_target_account should be removed from params")
		})
	}
}

func TestRawToolInfo_ConvertedSchema(t *testing.T) {
	t.Parallel()

	// Setup: reusable components
	userID := ids.RandomUserID()
	serverID := ids.RandomServerID()

	validParams := json.RawMessage(`{"type": "object", "properties": {"text": {"type": "string"}}}`)
	validResponse := json.RawMessage(`{"type": "object"}`)

	tests := []struct {
		name                     string
		setup                    func() RawToolInfo
		expectTargetAccountField bool
		expectedEnumValues       []string // only checked if expectTargetAccountField is true
		wantErr                  require.ErrorAssertionFunc
	}{{
		name: "single account - no _target_account field",
		setup: func() RawToolInfo {
			accountID := must(ids.RandomAccountID(userID, serverID))
			toolID := must(ids.RandomToolID(accountID))
			return must(NewRawToolInfo(
				"send_message",
				"Send a message",
				validParams,
				validResponse,
				WithMergedTool(toolID, "main_bot", "Main bot"),
			))
		},
		expectTargetAccountField: false,
	}, {
		name: "multiple accounts - _target_account field added with enum",
		setup: func() RawToolInfo {
			account1 := must(ids.RandomAccountID(userID, serverID))
			account2 := must(ids.RandomAccountID(userID, serverID))
			toolID1 := must(ids.RandomToolID(account1))
			toolID2 := must(ids.RandomToolID(account2))
			return must(NewRawToolInfo(
				"send_message",
				"Send a message",
				validParams,
				validResponse,
				WithMergedTool(toolID1, "main_bot", "Main production bot"),
				WithMergedTool(toolID2, "backup_bot", "Backup bot"),
			))
		},
		expectTargetAccountField: true,
		expectedEnumValues:       []string{"backup_bot", "main_bot"}, // sorted alphabetically
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tool := tt.setup()
			schemaBytes := tool.ConvertedSchema()

			// Parse schema to validate structure
			var schema map[string]interface{}
			require.NoError(t, json.Unmarshal(schemaBytes, &schema))

			properties, ok := schema["properties"].(map[string]interface{})
			require.True(t, ok, "schema should have 'properties' field")

			if tt.expectTargetAccountField {
				// Verify _target_account field exists
				targetAccountField, exists := properties["_target_account"]
				require.True(t, exists, "_target_account field should be present for multi-account tool")

				// Verify it's an enum
				field := targetAccountField.(map[string]interface{})
				require.Equal(t, "string", field["type"], "_target_account should be a string")

				enum, ok := field["enum"].([]interface{})
				require.True(t, ok, "_target_account should have enum values")

				// Convert to string slice for comparison
				gotEnumValues := make([]string, len(enum))
				for i, v := range enum {
					gotEnumValues[i] = v.(string)
				}
				require.Equal(t, tt.expectedEnumValues, gotEnumValues, "enum values should match")

				// Verify description exists
				desc, ok := field["description"].(string)
				require.True(t, ok, "_target_account should have description")
				require.NotEmpty(t, desc, "description should not be empty")

				// Verify it's in required fields
				required, ok := schema["required"].([]interface{})
				require.True(t, ok, "schema should have 'required' field")

				found := false
				for _, r := range required {
					if r.(string) == "_target_account" {
						found = true
						break
					}
				}
				require.True(t, found, "_target_account should be in required fields")
			} else {
				// Verify _target_account field does NOT exist
				_, exists := properties["_target_account"]
				require.False(t, exists, "_target_account field should NOT be present for single-account tool")

				// Verify original params are unchanged
				_, textExists := properties["text"]
				require.True(t, textExists, "original 'text' field should be present")
			}
		})
	}
}

func TestRawToolInfo_Merge(t *testing.T) {
	t.Parallel()

	userID := ids.RandomUserID()
	serverID := ids.RandomServerID()

	validParams := json.RawMessage(`{"type": "object", "properties": {"text": {"type": "string"}}}`)
	validResponse := json.RawMessage(`{"type": "object"}`)

	tests := []struct {
		name    string
		setup   func() (RawToolInfo, RawToolInfo)
		verify  func(*testing.T, RawToolInfo)
		wantErr require.ErrorAssertionFunc
	}{{
		name: "success: merge same schemas with different accounts",
		setup: func() (RawToolInfo, RawToolInfo) {
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

			return tool1, tool2
		},
		verify: func(t *testing.T, merged RawToolInfo) {
			require.Equal(t, 2, len(merged.EncodedTools()), "should have 2 accounts after merge")
		},
	}, {
		name: "error: different names",
		setup: func() (RawToolInfo, RawToolInfo) {
			accountID := must(ids.RandomAccountID(userID, serverID))
			toolID1 := must(ids.RandomToolID(accountID))
			toolID2 := must(ids.RandomToolID(accountID))

			tool1 := must(NewRawToolInfo("send_message", "Desc", validParams, validResponse, WithMergedTool(toolID1, "main_bot", "Bot")))
			tool2 := must(NewRawToolInfo("delete_message", "Desc", validParams, validResponse, WithMergedTool(toolID2, "backup_bot", "Bot")))

			return tool1, tool2
		},
		wantErr: require.Error,
	}, {
		name: "error: different descriptions",
		setup: func() (RawToolInfo, RawToolInfo) {
			accountID := must(ids.RandomAccountID(userID, serverID))
			toolID1 := must(ids.RandomToolID(accountID))
			toolID2 := must(ids.RandomToolID(accountID))

			tool1 := must(NewRawToolInfo("send_message", "Send messages", validParams, validResponse, WithMergedTool(toolID1, "main_bot", "Bot")))
			tool2 := must(NewRawToolInfo("send_message", "Delete messages", validParams, validResponse, WithMergedTool(toolID2, "backup_bot", "Bot")))

			return tool1, tool2
		},
		wantErr: require.Error,
	}, {
		name: "error: different params",
		setup: func() (RawToolInfo, RawToolInfo) {
			accountID := must(ids.RandomAccountID(userID, serverID))
			toolID := must(ids.RandomToolID(accountID))

			differentParams := json.RawMessage(`{"type": "object", "properties": {"message": {"type": "string"}}}`)

			tool1 := must(NewRawToolInfo("send_message", "Desc", validParams, validResponse, WithMergedTool(toolID, "main_bot", "Bot")))
			tool2 := must(NewRawToolInfo("send_message", "Desc", differentParams, validResponse, WithMergedTool(toolID, "main_bot", "Bot")))

			return tool1, tool2
		},
		wantErr: require.Error,
	}, {
		name: "error: different response",
		setup: func() (RawToolInfo, RawToolInfo) {
			accountID := must(ids.RandomAccountID(userID, serverID))
			toolID := must(ids.RandomToolID(accountID))

			differentResponse := json.RawMessage(`{"type": "object", "properties": {"success": {"type": "boolean"}}}`)

			tool1 := must(NewRawToolInfo("send_message", "Desc", validParams, validResponse, WithMergedTool(toolID, "main_bot", "Bot")))
			tool2 := must(NewRawToolInfo("send_message", "Desc", validParams, differentResponse, WithMergedTool(toolID, "main_bot", "Bot")))

			return tool1, tool2
		},
		wantErr: require.Error,
	}, {
		name: "error: duplicate tool id with different descriptions",
		setup: func() (RawToolInfo, RawToolInfo) {
			accountID := must(ids.RandomAccountID(userID, serverID))
			toolUUID := uuid.New()
			toolID := must(ids.NewToolID(accountID, toolUUID))

			tool1 := must(NewRawToolInfo("send_message", "Desc", validParams, validResponse, WithMergedTool(toolID, "main_bot", "Main bot")))
			tool2 := must(NewRawToolInfo("send_message", "Desc", validParams, validResponse, WithMergedTool(toolID, "backup_bot", "Backup bot")))

			return tool1, tool2
		},
		wantErr: require.Error,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			wantErr := noErrAsDefault(tt.wantErr)

			tool1, tool2 := tt.setup()
			merged, err := tool1.Merge(tool2)
			if wantErr(t, err); err != nil {
				return
			}

			require.True(t, merged.Valid(), "merged tool should be valid")
			if tt.verify != nil {
				tt.verify(t, merged)
			}
		})
	}
}

func TestRawToolInfo_Immutability(t *testing.T) {
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

	t.Run("Params returns cloned slice", func(t *testing.T) {
		// Get params twice
		params1 := tool.Params()
		params2 := tool.Params()

		// Modify first slice
		if len(params1) > 0 {
			params1[0] = 0xFF
		}

		// Second slice should be unaffected
		require.Equal(t, validParams, params2, "modification should not affect second slice")
	})

	t.Run("Response returns cloned slice", func(t *testing.T) {
		// Get response twice
		resp1 := tool.Response()
		resp2 := tool.Response()

		// Modify first slice
		if len(resp1) > 0 {
			resp1[0] = 0xFF
		}

		// Second slice should be unaffected
		require.Equal(t, validResponse, resp2, "modification should not affect second slice")
	})
}

func noErrAsDefault(fn require.ErrorAssertionFunc) require.ErrorAssertionFunc {
	if fn != nil {
		return fn
	}
	return require.NoError
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

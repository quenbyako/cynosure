// TODO: reduce tests complexity, for now they're overcomplicated for no reason

package tools_test

import (
	"embed"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"

	. "github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
)

//go:embed testdata/*.yaml
var testData embed.FS

func TestToolbox_Merge(t *testing.T) {
	t.Parallel()

	data := must[[]byte](t)(testData.ReadFile("testdata/toolbox_merge.yaml"))

	var suite struct {
		Cases []toolboxMergeYAMLCase `yaml:"cases"`
	}

	require.NoError(t, yaml.Unmarshal(data, &suite))

	userID := ids.RandomUserID()
	serverID := ids.RandomServerID()

	for _, tt := range suite.Cases {
		tt.Run(t, userID, serverID)
	}
}

type toolboxMergeYAMLCase struct {
	Name         string                  `yaml:"name"`
	InitialTools []mergeTestCaseToolDesc `yaml:"initialTools"`
	ToolsToAdd   []mergeTestCaseToolDesc `yaml:"toolsToAdd"`
	WantLen      int                     `yaml:"wantLen"`
	ExpectErr    bool                    `yaml:"expectErr"`
}

func (c *toolboxMergeYAMLCase) Run(t *testing.T, userID ids.UserID, serverID ids.ServerID) {
	t.Helper()

	t.Run(c.Name, func(t *testing.T) {
		t.Parallel()

		initial := c.setupInitial(t, userID, serverID)
		added := c.setupAdded(t, userID, serverID)

		merged, err := initial.Merge(added...)
		if c.ExpectErr {
			require.Error(t, err)

			return
		}

		require.NoError(t, err)
		require.True(t, merged.Valid())
		require.Len(t, merged.Tools(), c.WantLen)
	})
}

func (c *toolboxMergeYAMLCase) setupInitial(
	t *testing.T, userID ids.UserID, serverID ids.ServerID,
) Toolbox {
	t.Helper()

	toolbox := NewToolbox()

	for _, desc := range c.InitialTools {
		tool := must[RawTool](t)(createToolFromDesc(t, userID, serverID, desc))
		toolbox = must[Toolbox](t)(toolbox.Merge(tool))
	}

	return toolbox
}

func (c *toolboxMergeYAMLCase) setupAdded(
	t *testing.T, userID ids.UserID, serverID ids.ServerID,
) []RawTool {
	t.Helper()

	tools := make([]RawTool, 0, len(c.ToolsToAdd))

	for _, desc := range c.ToolsToAdd {
		tool, err := createToolFromDesc(t, userID, serverID, desc)
		if err != nil {
			tools = append(tools, RawTool{})
		} else {
			tools = append(tools, tool)
		}
	}

	return tools
}

func createToolFromDesc(
	t *testing.T,
	userID ids.UserID,
	serverID ids.ServerID,
	desc mergeTestCaseToolDesc,
) (RawTool, error) {
	t.Helper()

	params := must[json.RawMessage](t)(json.Marshal(desc.Params))
	resp := must[json.RawMessage](t)(json.Marshal(desc.Response))

	accID := must[ids.AccountID](t)(ids.RandomAccountID(userID, serverID))
	toolID := must[ids.ToolID](t)(ids.RandomToolID(accID))

	return NewRawTool(
		desc.Name, desc.Desc, params, resp,
		toolID, desc.AccountName, desc.AccountDesc,
	)
}

func TestToolbox_ConvertRequest(t *testing.T) {
	t.Parallel()

	data := must[[]byte](t)(testData.ReadFile("testdata/toolbox_convert.yaml"))

	var suite struct {
		Cases []toolboxConvertYAMLCase `yaml:"cases"`
	}

	require.NoError(t, yaml.Unmarshal(data, &suite))

	userID := ids.RandomUserID()
	serverID := ids.RandomServerID()

	for _, tt := range suite.Cases {
		tt.Run(t, userID, serverID)
	}
}

type toolboxConvertYAMLCase struct {
	Name           string                  `yaml:"name"`
	ToolboxTools   []mergeTestCaseToolDesc `yaml:"toolboxTools"`
	ToolName       string                  `yaml:"toolName"`
	Request        map[string]any          `yaml:"request"`
	ExpectedParams map[string]any          `yaml:"expectedParams"`
	ExpectErr      bool                    `yaml:"expectErr"`
}

func (c *toolboxConvertYAMLCase) Run(t *testing.T, userID ids.UserID, serverID ids.ServerID) {
	t.Helper()
	t.Run(c.Name, func(t *testing.T) {
		t.Parallel()

		toolbox := c.setupToolbox(t, userID, serverID)
		request := c.setupRequest(t)

		gotID, gotParams, err := toolbox.ConvertRequest(c.ToolName, request)
		if c.ExpectErr {
			require.Error(t, err)

			return
		}

		require.NoError(t, err)

		// We need to find the correct tool ID since setupToolbox generates random ones
		expID := toolbox.Tools()[c.ToolName].EncodedTools()

		var toolID ids.ToolID

		for id := range expID {
			toolID = id
		}

		require.Equal(t, toolID, gotID)

		expParams := c.setupExpectedParams(t)

		require.Equal(t, expParams, gotParams)
	})
}

func (c *toolboxConvertYAMLCase) setupToolbox(
	t *testing.T, userID ids.UserID, serverID ids.ServerID,
) Toolbox {
	t.Helper()

	toolbox := NewToolbox()

	for _, desc := range c.ToolboxTools {
		tool := must[RawTool](t)(createToolFromDesc(t, userID, serverID, desc))
		toolbox = must[Toolbox](t)(toolbox.Merge(tool))
	}

	return toolbox
}

func (c *toolboxConvertYAMLCase) setupRequest(t *testing.T) map[string]json.RawMessage {
	t.Helper()

	req := make(map[string]json.RawMessage)

	for k, v := range c.Request {
		req[k] = must[json.RawMessage](t)(json.Marshal(v))
	}

	return req
}

func (c *toolboxConvertYAMLCase) setupExpectedParams(t *testing.T) map[string]json.RawMessage {
	t.Helper()

	exp := make(map[string]json.RawMessage)

	for k, v := range c.ExpectedParams {
		exp[k] = must[json.RawMessage](t)(json.Marshal(v))
	}

	return exp
}

func TestToolbox_Immutability(t *testing.T) {
	t.Parallel()

	userID := ids.RandomUserID()
	serverID := ids.RandomServerID()
	accID, err := ids.RandomAccountID(userID, serverID)
	require.NoError(t, err)
	tID, err := ids.RandomToolID(accID)
	require.NoError(t, err)

	tool, err := NewRawTool("send_message", "Desc",
		json.RawMessage(`{"type": "object"}`),
		json.RawMessage(`{"type": "object"}`),
		tID, "bot", "Bot")
	require.NoError(t, err)

	toolbox, err := NewToolbox().Merge(tool)
	require.NoError(t, err)

	t.Run("Tools returns cloned map", func(t *testing.T) {
		t.Parallel()

		tools1 := toolbox.Tools()
		tools2 := toolbox.Tools()

		delete(tools1, "send_message")

		_, exists := tools2["send_message"]
		require.True(t, exists)
	})
}

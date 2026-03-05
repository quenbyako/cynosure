// TODO: optimize tests here. Super complicated, this complexity makes no sense.

package tools_test

import (
	"embed"
	"encoding/json"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"

	. "github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
)

//go:embed testdata/*
var testdata embed.FS

func TestRawToolInfo_Merge(t *testing.T) {
	t.Parallel()

	data, err := testdata.ReadFile("testdata/merge.yaml")
	require.NoError(t, err)

	var suite struct {
		Cases []mergeTestCase `yaml:"cases"`
	}
	require.NoError(t, yaml.Unmarshal(data, &suite))

	userID := ids.RandomUserID()
	serverID := ids.RandomServerID()

	for _, tt := range suite.Cases {
		tt.Run(t, userID, serverID)
	}
}

type (
	mergeTestCase struct {
		Name          string                  `yaml:"name"`
		Tools         []mergeTestCaseToolDesc `yaml:"tools"`
		ExpectErr     bool                    `yaml:"expectErr"`
		ExpectedCount int                     `yaml:"expectedCount"`
	}

	mergeTestCaseToolDesc struct {
		Name        string `yaml:"name"`
		Desc        string `yaml:"desc"`
		Params      any    `yaml:"params"`
		Response    any    `yaml:"response"`
		AccountName string `yaml:"accountName"`
		AccountDesc string `yaml:"accountDesc"`
	}
)

func (c *mergeTestCase) Run(t *testing.T, userID ids.UserID, serverID ids.ServerID) {
	t.Helper()

	t.Run(c.Name, func(t *testing.T) {
		t.Parallel()

		tools := make([]RawTool, 0, len(c.Tools))

		for _, toolDesc := range c.Tools {
			tool, err := c.createTool(t, userID, serverID, toolDesc)
			require.NoError(t, err)

			tools = append(tools, tool)
		}

		merged, err := MergeTools(tools...)
		c.wantErr(t, err)

		if err != nil {
			return
		}

		require.True(t, merged.Valid())
		require.Len(t, merged.EncodedTools(), c.ExpectedCount)
	})
}

func (c *mergeTestCase) createTool(
	t *testing.T, userID ids.UserID, serverID ids.ServerID, tool mergeTestCaseToolDesc,
) (RawTool, error) {
	t.Helper()

	accountID := must[ids.AccountID](t)(ids.RandomAccountID(userID, serverID))
	toolID := must[ids.ToolID](t)(ids.RandomToolID(accountID))

	paramsBytes := must[json.RawMessage](t)(json.Marshal(tool.Params))
	responseBytes := must[json.RawMessage](t)(json.Marshal(tool.Response))

	return NewRawTool(
		tool.Name, tool.Desc, paramsBytes, responseBytes,
		toolID, tool.AccountName, tool.AccountDesc,
	)
}

func (c *mergeTestCase) wantErr(testingT require.TestingT, err error, msgAndArgs ...any) {
	if helper, ok := testingT.(interface{ Helper() }); ok {
		helper.Helper()
	}

	if c.ExpectErr {
		require.Error(testingT, err, msgAndArgs...)

		return
	}

	require.NoError(testingT, err, msgAndArgs...)
}

func TestRawTool_ParamsImmutability(t *testing.T) {
	tool := validRawTool(t)
	original := slices.Clone(tool.Params())

	params1 := tool.Params()
	params2 := tool.Params()

	if len(params1) > 0 {
		params1[0] = 0xFF
	}

	require.Equal(t, original, params2)
	require.NotEqual(t, params1, params2)
}

func TestRawTool_ResponseImmutability(t *testing.T) {
	tool := validRawTool(t)
	original := slices.Clone(tool.Response())

	resp1 := tool.Response()
	resp2 := tool.Response()

	if len(resp1) > 0 {
		resp1[0] = 0xFF
	}

	require.Equal(t, original, resp2)
	require.NotEqual(t, resp1, resp2)
}

func validRawTool(t *testing.T) RawTool {
	t.Helper()

	return must[RawTool](t)(NewRawTool(
		"send_message",
		"Desc",
		json.RawMessage(`{"type": "object"}`),
		json.RawMessage(`{"type": "object"}`),
		RandomToolID(t),
		"main_tool",
		"Main Description",
	))
}

func RandomToolID(t *testing.T) ids.ToolID {
	t.Helper()

	user := ids.RandomUserID()
	server := ids.RandomServerID()
	account := must[ids.AccountID](t)(ids.RandomAccountID(user, server))

	return must[ids.ToolID](t)(ids.RandomToolID(account))
}

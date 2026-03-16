package testsuite

import (
	"encoding/json"
	"errors"
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/messages"
)

const embeddingSize = 1536

type ToolSemanticIndexTestSuite struct {
	adapter ports.ToolSemanticIndex
}

type ToolSemanticIndexTestSuiteOpts func(*ToolSemanticIndexTestSuite)

// RunToolSemanticIndexTests runs tests for the given ToolSemanticIndex adapter.
// These tests are predefined and recommended to be used for ANY adapter implementation.
//
// After constructing test function, just run it through `run(t)` call,
// everything else will be handled for you. Calling this function through
// `t.Run("general", run)` is not very recommended, cause test logs will be too
// hard to read cause of big nesting.
func RunToolSemanticIndexTests(
	a ports.ToolSemanticIndex, opts ...ToolSemanticIndexTestSuiteOpts,
) func(t *testing.T) {
	suite := &ToolSemanticIndexTestSuite{
		adapter: a,
	}
	for _, opt := range opts {
		opt(suite)
	}

	if err := suite.validate(); err != nil {
		panic(err)
	}

	return runSuite(suite)
}

func (s *ToolSemanticIndexTestSuite) validate() error {
	if s.adapter == nil {
		return errors.New("adapter is nil")
	}

	return nil
}

// TestIndexTool verifies that IndexTool generates valid embeddings for various tool configurations.
func (s *ToolSemanticIndexTestSuite) TestIndexTool(t *testing.T) {
	tests := []struct {
		buildTool func(t *testing.T) entities.ToolReadOnly
		name      string
	}{{
		name:      "simple_tool",
		buildTool: s.buildSimpleTool,
	}, {
		name:      "complex_schema",
		buildTool: s.buildComplexTool,
	}, {
		name:      "empty_description",
		buildTool: s.buildToolEmptyDesc,
	}, {
		name:      "minimal_schema",
		buildTool: s.buildMinimalTool,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := tt.buildTool(t)

			embedding, err := s.adapter.IndexTool(t.Context(), tool)
			require.NoError(t, err, "IndexTool should not fail")

			s.assertValidEmbedding(t, embedding)
		})
	}
}

// TestBuildToolEmbedding verifies that BuildToolEmbedding handles various message combinations.
func (s *ToolSemanticIndexTestSuite) TestBuildToolEmbedding(t *testing.T) {
	tests := []struct {
		name     string
		messages []messages.Message
	}{{
		name:     "empty_messages",
		messages: []messages.Message{},
	}, {
		name: "user_only",
		messages: []messages.Message{
			must(messages.NewMessageUser("Hello")),
			must(messages.NewMessageUser("What can you do?")),
		},
	}, {
		name: "user_and_assistant",
		messages: []messages.Message{
			must(messages.NewMessageUser("What's the weather?")),
			must(messages.NewMessageAssistant("Let me check that for you.")),
		},
	}, {
		name: "full_tool_interaction",
		messages: []messages.Message{
			must(messages.NewMessageUser("What's the weather in New York?")),
			must(messages.NewMessageAssistant("Let me check that for you.")),
			must(messages.NewMessageToolRequest(
				map[string]json.RawMessage{"location": json.RawMessage(`"New York"`)},
				"get_weather",
				"call_123",
			)),
			must(messages.NewMessageToolResponse(
				json.RawMessage(`{"temperature": 72, "condition": "sunny"}`),
				"get_weather",
				"call_123",
			)),
		},
	}, {
		name: "tool_error_handling",
		messages: []messages.Message{
			must(messages.NewMessageUser("Get weather")),
			must(messages.NewMessageToolRequest(
				map[string]json.RawMessage{"location": json.RawMessage(`"Invalid"`)},
				"get_weather",
				"call_456",
			)),
			must(messages.NewMessageToolError(
				json.RawMessage(`{"error": "Invalid location"}`),
				"get_weather",
				"call_456",
			)),
		},
	}, {
		name: "multiple_tool_calls",
		messages: []messages.Message{
			must(messages.NewMessageUser("Get weather and time")),
			must(messages.NewMessageToolRequest(
				map[string]json.RawMessage{"location": json.RawMessage(`"NYC"`)},
				"get_weather",
				"call_1",
			)),
			must(messages.NewMessageToolRequest(
				map[string]json.RawMessage{"timezone": json.RawMessage(`"EST"`)},
				"get_time",
				"call_2",
			)),
			must(messages.NewMessageToolResponse(
				json.RawMessage(`{"temperature": 70}`),
				"get_weather",
				"call_1",
			)),
			must(messages.NewMessageToolResponse(
				json.RawMessage(`{"time": "14:30"}`),
				"get_time",
				"call_2",
			)),
		},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			embedding, err := s.adapter.BuildToolEmbedding(t.Context(), tt.messages)
			require.NoError(t, err, "BuildToolEmbedding should not fail")

			s.assertValidEmbedding(t, embedding)
		})
	}
}

// Helper: assertValidEmbedding checks that embedding is valid (correct size, non-zero).
func (s *ToolSemanticIndexTestSuite) assertValidEmbedding(
	t *testing.T, embedding [embeddingSize]float32,
) {
	t.Helper()

	// Check that at least some values are non-zero (embedding is not all zeros)
	hasNonZero := false

	for _, v := range &embedding {
		if v != 0 {
			hasNonZero = true
			break
		}
	}

	require.True(t, hasNonZero, "should have at least some non-zero values")

	// Check that all values are finite (not NaN or Inf)
	for i, v := range &embedding {
		require.False(t, math.IsNaN(float64(v)), "value at index %d is NaN", i)
		require.False(t, math.IsInf(float64(v), 0), "value at index %d is Inf", i)
	}
}

// buildSimpleTool creates a simple tool for testing.
func (s *ToolSemanticIndexTestSuite) buildSimpleTool(t *testing.T) entities.ToolReadOnly {
	t.Helper()

	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"location": {
				"type": "string",
				"description": "City name"
			}
		},
		"required": ["location"]
	}`)

	responseSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"temperature": {"type": "number"},
			"condition": {"type": "string"}
		}
	}`)

	return s.buildTool(
		t, "get_weather", "Get current weather for a location", schema, responseSchema,
	)
}

// buildComplexTool creates a tool with complex nested schema.
func (s *ToolSemanticIndexTestSuite) buildComplexTool(t *testing.T) entities.ToolReadOnly {
	t.Helper()

	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {
				"type": "object",
				"properties": {
					"filters": {
						"type": "array",
						"items": {
							"type": "object",
							"properties": {
								"field": {"type": "string"},
								"operator": {"type": "string"},
								"value": {}
							}
						}
					},
					"sort": {
						"type": "object",
						"properties": {
							"field": {"type": "string"},
							"order": {"type": "string", "enum": ["asc", "desc"]}
						}
					}
				}
			}
		}
	}`)

	responseSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"results": {
				"type": "array",
				"items": {"type": "object"}
			},
			"total": {"type": "number"}
		}
	}`)

	return s.buildTool(
		t, "search_database", "Search database with complex filters and sorting",
		schema, responseSchema,
	)
}

// buildToolEmptyDesc creates a tool with empty description.
func (s *ToolSemanticIndexTestSuite) buildToolEmptyDesc(t *testing.T) entities.ToolReadOnly {
	t.Helper()

	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"input": {"type": "string"}
		}
	}`)

	responseSchema := json.RawMessage(`{"type": "object"}`)

	return s.buildTool(t, "mystery_tool", "", schema, responseSchema)
}

// buildMinimalTool creates a tool with minimal schema.
func (s *ToolSemanticIndexTestSuite) buildMinimalTool(t *testing.T) entities.ToolReadOnly {
	t.Helper()

	schema := json.RawMessage(`{"type": "object"}`)
	responseSchema := json.RawMessage(`{"type": "string"}`)

	return s.buildTool(t, "minimal_tool", "A minimal tool", schema, responseSchema)
}

// buildTool is a helper for creating tools with given parameters.
func (s *ToolSemanticIndexTestSuite) buildTool(
	t *testing.T,
	name, description string,
	schema, responseSchema json.RawMessage,
) entities.ToolReadOnly {
	t.Helper()

	account := must(ids.RandomAccountID(ids.RandomUserID(), ids.RandomServerID()))

	tool, err := entities.NewTool(
		must(ids.RandomToolID(account)),
		"test-account",
		name,
		description,
		schema,
		responseSchema,
	)

	require.NoError(t, err, "failed to create tool: %q", name)

	return tool
}

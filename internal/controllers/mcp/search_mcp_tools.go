package mcp

import "context"

type (
	SearchMcpToolsInput struct {
		Query string `json:"query" jsonschema:"Search query text"`
		Limit int    `json:"limit,omitempty" jsonschema:"Max results"`
	}

	SearchMcpToolsOutput struct {
		Tools []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"tools"`
	}
)

func (c *Controller) SearchMcpTools(_ context.Context, in SearchMcpToolsInput) (SearchMcpToolsOutput, error) {
	userID := userID // TODO: get it from context
	_ = userID

	return SearchMcpToolsOutput{}, nil
}

package mcp

import "context"

type (
	SearchMcpServersInput struct {
		Query string `json:"query" jsonschema:"Search query text"`
		Limit int    `json:"limit,omitempty" jsonschema:"Maximum number of results (default: 10)"`
	}
	SearchMcpServersOutput struct {
		Servers []struct {
			URL  string `json:"url"`
			Name string `json:"name"`
		} `json:"servers"`
	}
)

func (c *Controller) SearchMcpServers(_ context.Context, in SearchMcpServersInput) (SearchMcpServersOutput, error) {
	userID := userID // TODO: get it from context
	_ = userID
	
	return SearchMcpServersOutput{}, nil
}

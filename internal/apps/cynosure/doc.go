// Package cynosure provides cynosure application.
package cynosure

import "github.com/modelcontextprotocol/go-sdk/mcp"

const (
	defaultWorkersCount = 4
	DefaultHistoryLimit = 50
)

var mcpImpl = mcp.Implementation{
	Name:       "admin-mcp-server",
	Title:      "Admin MCP Server",
	Version:    "1.0.0",
	WebsiteURL: "https://t.me/zhopakotabot",
	Icons:      nil,
}

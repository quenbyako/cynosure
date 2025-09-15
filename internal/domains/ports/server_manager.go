package ports

import (
	"context"
	"net/url"
	"time"

	"golang.org/x/oauth2"

	"tg-helper/internal/domains/components/ids"
)

type ServerStorage interface {
	AddServer(ctx context.Context, name ids.ServerID, info ServerInfo) error
	ListServers(ctx context.Context, limit uint, token string) (m map[ids.ServerID]ServerInfo, nextToken string, err error)
	GetServerInfo(ctx context.Context, name ids.ServerID) (*ServerInfo, error)
	LookupByURL(ctx context.Context, url *url.URL) (ids.ServerID, ServerInfo, error)
}

type ServerInfo struct {
	SSELink    *url.URL
	AuthConfig *oauth2.Config
	// If expiration is empty — probably config works indefinitely
	ConfigExpiration time.Time
}

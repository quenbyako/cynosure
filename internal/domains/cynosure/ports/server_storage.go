package ports

import (
	"context"
	"net/url"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/ids"
)

type ServerStorage interface {
	ServerStorageRead
	ServerStorageWrite
}

type ServerStorageRead interface {
	ListServers(ctx context.Context) (m []*entities.ServerConfig, err error)
	GetServerInfo(ctx context.Context, name ids.ServerID) (*entities.ServerConfig, error)
	LookupByURL(ctx context.Context, url *url.URL) (*entities.ServerConfig, error)
}

type ServerStorageWrite interface {
	AddServer(ctx context.Context, config entities.ServerConfigReadOnly) error
}

type ServerStorageFactory interface {
	ServerStorage() ServerStorage
}

func NewServerStorage(factory ServerStorageFactory) ServerStorage { return factory.ServerStorage() }

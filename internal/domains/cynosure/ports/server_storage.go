package ports

import (
	"context"
	"net/url"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

// ServerStorage manages persistence of MCP server configurations.
// Each server config defines connection details (URL), OAuth settings, and
// expiration for server discovery caching.
type ServerStorage interface {
	ServerStorageRead
	ServerStorageWrite
}

type ServerStorageRead interface {
	// ListServers returns all known MCP server configurations. Empty slice if
	// none exist.
	//
	// See next test suites to find how it works:
	//
	//  -  [TestSaveServer] — listing servers and verifying saved server is
	//     present
	ListServers(ctx context.Context) (m []*entities.ServerConfig, err error)

	// GetServerInfo retrieves server configuration by ID.
	//
	// See next test suites to find how it works:
	//
	//  -  [TestSaveServer] — retrieving server and verifying ID matches
	//
	// Throws:
	//
	//  - [ErrNotFound] if server doesn't exist.
	GetServerInfo(ctx context.Context, name ids.ServerID) (*entities.ServerConfig, error)

	// LookupByURL finds server configuration by its connection URL. Useful for
	// deduplication when adding servers discovered from multiple sources.
	//
	// See next test suites to find how it works:
	//
	//  - [TestLookupByURL] — finding servers by URL for deduplication
	//
	// Throws:
	//
	//  - [ErrNotFound] if no server with this URL exists.
	LookupByURL(ctx context.Context, url *url.URL) (*entities.ServerConfig, error)
}

type ServerStorageWrite interface {
	// SetServer creates or updates server configuration (upsert). Idempotent
	// operation. Does not validate URL reachability or OAuth configuration -
	// validation happens in domain layer.
	//
	// See next test suites to find how it works:
	//
	//  -  [TestSaveServer] — persisting server config and retrieving it back
	SetServer(ctx context.Context, config entities.ServerConfigReadOnly) error

	// DeleteServer removes server configuration by ID. Idempotent operation.
	// Does not cascade to accounts using this server - accounts will fail to
	// connect until re-configured.
	//
	// See next test suites to find how it works:
	//
	//  - [TestDeleteServer] — removing server and verifying cascade behavior
	DeleteServer(ctx context.Context, id ids.ServerID) error
}

type ServerStorageFactory interface {
	ServerStorage() ServerStorage
}

//nolint:ireturn // standard port pattern: hiding implementation details
func NewServerStorage(factory ServerStorageFactory) ServerStorage { return factory.ServerStorage() }

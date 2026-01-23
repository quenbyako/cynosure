package sql

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/quenbyako/cynosure/contrib/db/gen/go"
	"github.com/quenbyako/cynosure/internal/adapters/sql/datatransfer"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/ids"
)

var _ ports.ServerStorage = (*Adapter)(nil)

// AddServer implements ServerStorage.
func (a *Adapter) AddServer(ctx context.Context, server entities.ServerConfigReadOnly) error {
	tx, err := a.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	q := a.q.WithTx(tx)

	// Insert the server record
	if err := q.AddServer(ctx, db.AddServerParams{
		ID:  server.ID().ID(),
		Url: server.SSELink().String(),
	}); err != nil {
		return fmt.Errorf("failed to add server: %w", err)
	}

	// If auth config is provided, insert it as well
	if server.AuthConfig() == nil {
		return nil
	}
	// Convert scopes array (may be nil)
	scopes := server.AuthConfig().Scopes
	if scopes == nil {
		scopes = []string{}
	}

	// Convert expiration to pgtype.Timestamp
	var expiration pgtype.Timestamp
	if !server.ConfigExpiration().IsZero() {
		expiration = pgtype.Timestamp{
			Time:  server.ConfigExpiration(),
			Valid: true,
		}
	}

	if err := q.AddOAuthConfig(ctx, db.AddOAuthConfigParams{
		ServerID:     server.ID().ID(),
		ClientID:     server.AuthConfig().ClientID,
		ClientSecret: server.AuthConfig().ClientSecret,
		RedirectUrl:  server.AuthConfig().RedirectURL,
		AuthUrl:      server.AuthConfig().Endpoint.AuthURL,
		TokenUrl:     server.AuthConfig().Endpoint.TokenURL,
		Expiration:   expiration,
		Scopes:       scopes,
	}); err != nil {
		return fmt.Errorf("failed to add oauth config: %w", err)
	}

	return nil
}

// ListServers implements ServerStorage.
func (a *Adapter) ListServers(ctx context.Context) ([]*entities.ServerConfig, error) {
	rows, err := a.q.ListServers(ctx, db.ListServersParams{
		Limit:  100,
		Offset: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list servers: %w", err)
	}

	servers := make([]*entities.ServerConfig, 0, len(rows))
	for _, row := range rows {
		server, err := datatransfer.ServerInfoListFromDB(row)
		if err != nil {
			return nil, fmt.Errorf("failed to convert server info: %w", err)
		}

		servers = append(servers, server)
	}

	return servers, nil
}

// GetServerInfo implements ServerStorage.
func (a *Adapter) GetServerInfo(ctx context.Context, id ids.ServerID) (*entities.ServerConfig, error) {
	row, err := a.q.GetServerInfo(ctx, id.ID())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ports.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get server info: %w", err)
	}

	info, err := datatransfer.ServerInfoFromDB(row)
	if err != nil {
		return nil, fmt.Errorf("failed to convert server info: %w", err)
	}

	return info, nil
}

// LookupByURL implements ServerStorage.
func (a *Adapter) LookupByURL(ctx context.Context, u *url.URL) (*entities.ServerConfig, error) {
	row, err := a.q.LookupByURL(ctx, u.String())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ports.ErrNotFound
		}
		return nil, fmt.Errorf("failed to lookup server: %w", err)
	}

	info, err := datatransfer.ServerInfoFromDB(db.GetServerInfoRow{
		ID:           row.ID,
		Url:          row.Url,
		ClientID:     row.ClientID,
		ClientSecret: row.ClientSecret,
		RedirectUrl:  row.RedirectUrl,
		AuthUrl:      row.AuthUrl,
		TokenUrl:     row.TokenUrl,
		Expiration:   row.Expiration,
		Scopes:       row.Scopes,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to convert server info: %w", err)
	}

	return info, nil
}

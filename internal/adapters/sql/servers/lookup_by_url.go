package servers

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/jackc/pgx/v5"
	db "github.com/quenbyako/cynosure/contrib/db/gen/go"

	"github.com/quenbyako/cynosure/internal/adapters/sql/datatransfer"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
)

func (s *Servers) LookupByURL(ctx context.Context, u *url.URL) (*entities.ServerConfig, error) {
	row, err := s.q.LookupByURL(ctx, u.String())
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

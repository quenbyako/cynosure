package servers

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/quenbyako/cynosure/contrib/db/gen/go"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
)

func (s *Servers) SetServer(ctx context.Context, server entities.ServerConfigReadOnly) error {
	tx, err := s.tx.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	q := s.q.WithTx(tx)

	// Insert the server record
	if err := q.AddServer(ctx, db.AddServerParams{
		ID:        server.ID().ID(),
		Url:       server.SSELink().String(),
		Embedding: nil,
	}); err != nil {
		return fmt.Errorf("failed to add server: %w", err)
	}

	// If auth config is provided, insert it as well
	if server.AuthConfig() == nil {
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit tx: %w", err)
		}

		return nil
	}
	// Convert scopes array (may be nil)
	scopes := server.AuthConfig().Scopes
	if scopes == nil {
		scopes = []string{}
	}

	// Convert expiration to pgtype.Timestamptz
	var expiration pgtype.Timestamptz
	if !server.ConfigExpiration().IsZero() {
		expiration = pgtype.Timestamptz{
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

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

package servers

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/quenbyako/cynosure/contrib/db/gen/go"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
)

var emptyOptions pgx.TxOptions

func (s *Servers) SetServer(ctx context.Context, server entities.ServerConfigReadOnly) error {
	transaction, err := s.tx.BeginTx(ctx, emptyOptions)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	//nolint:errcheck // Rollback is called in defer and we often don't check its error
	defer transaction.Rollback(ctx)

	queries := s.q.WithTx(transaction)

	if err := queries.AddServer(ctx, db.AddServerParams{
		ID:        server.ID().ID(),
		Url:       server.SSELink().String(),
		Embedding: nil,
	}); err != nil {
		return fmt.Errorf("failed to add server: %w", err)
	}

	if err := s.addOAuthConfig(ctx, queries, server); err != nil {
		return err
	}

	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

func (s *Servers) addOAuthConfig(
	ctx context.Context,
	queries *db.Queries,
	server entities.ServerConfigReadOnly,
) error {
	if server.AuthConfig() == nil {
		return nil
	}

	scopes := server.AuthConfig().Scopes
	if scopes == nil {
		scopes = []string{}
	}

	if err := queries.AddOAuthConfig(ctx, db.AddOAuthConfigParams{
		ServerID:     server.ID().ID(),
		ClientID:     server.AuthConfig().ClientID,
		ClientSecret: server.AuthConfig().ClientSecret,
		RedirectUrl:  server.AuthConfig().RedirectURL,
		AuthUrl:      server.AuthConfig().Endpoint.AuthURL,
		TokenUrl:     server.AuthConfig().Endpoint.TokenURL,
		Expiration:   timeToTimestampz(server.ConfigExpiration()),
		Scopes:       scopes,
	}); err != nil {
		return fmt.Errorf("failed to add oauth config: %w", err)
	}

	return nil
}

func timeToTimestampz(value time.Time) (res pgtype.Timestamptz) {
	if value.IsZero() {
		return res
	}

	return pgtype.Timestamptz{
		Time:             value,
		InfinityModifier: pgtype.Finite,
		Valid:            true,
	}
}

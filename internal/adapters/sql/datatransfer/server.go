package datatransfer

import (
	"fmt"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/quenbyako/cynosure/contrib/db/gen/go"
	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/adapters/sql/errors"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
)

// ServerInfoFromDB converts database row to ServerConfig entity.
// This is pure type conversion - NO database queries.
func ServerInfoFromDB(row *db.GetServerInfoRow) (*entities.ServerConfig, error) {
	id, err := ids.NewServerID(row.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid server id: %w", err)
	}

	return mapServerConfig(
		id,
		row.Url,
		row.ClientID,
		row.ClientSecret,
		row.RedirectUrl,
		row.AuthUrl,
		row.TokenUrl,
		row.Scopes,
		timeFromPgTypestamp(row.Expiration),
	)
}

// ServerInfoListFromDB converts ListServersRow to ServerConfig entity.
// This is pure type conversion - NO database queries.
func ServerInfoListFromDB(row *db.ListServersRow) (*entities.ServerConfig, error) {
	serverID, err := ids.NewServerID(row.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid server id: %w", err)
	}

	return mapServerConfig(
		serverID,
		row.Url,
		row.ClientID,
		row.ClientSecret,
		row.RedirectUrl,
		row.AuthUrl,
		row.TokenUrl,
		row.Scopes,
		timeFromPgTypestamp(row.Expiration),
	)
}

func mapServerConfig(
	id ids.ServerID, urlStr string,
	clientID, clientSecret, redirectURL, authURL, tokenURL *string,
	scopes []string, expiration *time.Time,
) (*entities.ServerConfig, error) {
	sse, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("invalid sse url: %w", err)
	}

	auth, err := buildOAuthConfig(clientID, clientSecret, redirectURL, authURL, tokenURL, scopes)
	if err != nil {
		return nil, err
	}

	opts := []entities.ServerConfigOption{
		entities.WithAuthConfig(auth),
		entities.WithProtocol(tools.ProtocolUnknown),
	}

	if expiration != nil {
		opts = append(opts, entities.WithExpiration(*expiration))
	}

	cfg, err := entities.NewServerConfig(id, sse, opts...)
	if err != nil {
		return nil, fmt.Errorf("new server config: %w", err)
	}

	return cfg, nil
}

// buildOAuthConfig builds OAuth2 config from database row fields.
func buildOAuthConfig(
	clientID, clientSecret, redirectURL, authURL, tokenURL *string,
	scopes []string,
) (*oauth2.Config, error) {
	if clientID == nil || *clientID == "" {
		//nolint:nilnil // OAuth is optional, returning nil config is intended
		return nil, nil
	}

	if authURL == nil || *authURL == "" {
		return nil, errors.ErrOAuthAuthURLEmpty
	}

	if tokenURL == nil || *tokenURL == "" {
		return nil, errors.ErrOAuthTokenURLEmpty
	}

	return &oauth2.Config{
		ClientID:     *clientID,
		ClientSecret: deref(clientSecret),
		RedirectURL:  deref(redirectURL),
		Scopes:       scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:       *authURL,
			TokenURL:      *tokenURL,
			DeviceAuthURL: "",
			AuthStyle:     0,
		},
	}, nil
}

func timeFromPgTypestamp(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}

	cloned := t.Time

	return &cloned
}

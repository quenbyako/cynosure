package datatransfer

import (
	"errors"
	"fmt"
	"net/url"
	"time"

	db "github.com/quenbyako/cynosure/contrib/db/gen/go"
	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
)

// ServerInfoFromDB converts database row to ServerConfig entity.
// This is pure type conversion - NO database queries.
func ServerInfoFromDB(row db.GetServerInfoRow) (*entities.ServerConfig, error) {
	// Parse ID
	id, err := ids.NewServerID(row.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid server id: %w", err)
	}

	// Parse URL
	sseLink, err := url.Parse(row.Url)
	if err != nil {
		return nil, fmt.Errorf("invalid sse url: %w", err)
	}

	// Build OAuth2 config from database row
	authConfig, err := buildOAuthConfig(
		row.ClientID,
		row.ClientSecret,
		row.RedirectUrl,
		row.AuthUrl,
		row.TokenUrl,
		row.Scopes,
	)
	if err != nil {
		return nil, err
	}

	// Parse expiration
	var configExpiration time.Time
	if row.Expiration.Valid {
		configExpiration = row.Expiration.Time
	}

	opts := []entities.ServerConfigOption{
		entities.WithAuthConfig(authConfig),
		entities.WithExpiration(configExpiration),
	}

	// Protocol is not yet persisted, return default (invalid) value
	// which corresponds to "unknown" state
	opts = append(opts, entities.WithProtocol(tools.Protocol(0)))

	cfg, err := entities.NewServerConfig(id, sseLink, opts...)
	if err != nil {
		return nil, fmt.Errorf("new server config: %w", err)
	}

	return cfg, nil
}

// ServerInfoListFromDB converts ListServersRow to ServerConfig entity.
// This is pure type conversion - NO database queries.
func ServerInfoListFromDB(row db.ListServersRow) (*entities.ServerConfig, error) {
	serverID, err := ids.NewServerID(row.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid server id: %w", err)
	}

	// Parse URL
	sseLink, err := url.Parse(row.Url)
	if err != nil {
		return nil, fmt.Errorf("invalid sse url: %w", err)
	}

	// Build OAuth2 config from database row
	authConfig, err := buildOAuthConfig(
		row.ClientID,
		row.ClientSecret,
		row.RedirectUrl,
		row.AuthUrl,
		row.TokenUrl,
		row.Scopes,
	)
	if err != nil {
		return nil, err
	}

	// Parse expiration
	var configExpiration time.Time
	if row.Expiration.Valid {
		configExpiration = row.Expiration.Time
	}

	opts := []entities.ServerConfigOption{
		entities.WithAuthConfig(authConfig),
		entities.WithExpiration(configExpiration),
	}

	// Protocol is not yet persisted, return default (invalid) value
	// which corresponds to "unknown" state
	opts = append(opts, entities.WithProtocol(tools.Protocol(0)))

	cfg, err := entities.NewServerConfig(serverID, sseLink, opts...)
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
		return nil, errors.New("invalid oauth config: auth URL is empty")
	}

	if tokenURL == nil || *tokenURL == "" {
		return nil, errors.New("invalid oauth config: token URL is empty")
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

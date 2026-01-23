package datatransfer

import (
	"errors"
	"net/url"
	"time"

	db "github.com/quenbyako/cynosure/contrib/db/gen/go"
	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/ids"
)

// ServerInfoFromDB converts database row to ServerConfig entity.
// This is pure type conversion - NO database queries.
func ServerInfoFromDB(row db.GetServerInfoRow) (*entities.ServerConfig, error) {
	id, err := ids.NewServerID(row.ID)
	if err != nil {
		return nil, err
	}

	// Parse URL
	sseLink, err := url.Parse(row.Url)
	if err != nil {
		return nil, err
	}

	// Build OAuth2 config from database row
	authConfig, err := buildOAuthConfig(row.ClientID, row.ClientSecret, row.RedirectUrl, row.AuthUrl, row.TokenUrl, row.Scopes)
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

	return entities.NewServerConfig(id, sseLink, opts...)
}

// ServerInfoListFromDB converts ListServersRow to ServerConfig entity.
// This is pure type conversion - NO database queries.
func ServerInfoListFromDB(row db.ListServersRow) (*entities.ServerConfig, error) {
	serverID, err := ids.NewServerID(row.ID)
	if err != nil {
		return nil, err
	}

	// Parse URL
	sseLink, err := url.Parse(row.Url)
	if err != nil {
		return nil, err
	}

	// Build OAuth2 config from database row
	authConfig, err := buildOAuthConfig(row.ClientID, row.ClientSecret, row.RedirectUrl, row.AuthUrl, row.TokenUrl, row.Scopes)
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

	return entities.NewServerConfig(serverID, sseLink, opts...)
}

// buildOAuthConfig builds OAuth2 config from database row fields.
func buildOAuthConfig(clientID, clientSecret, redirectUrl, authUrl, tokenUrl *string, scopes []string) (*oauth2.Config, error) {
	if clientID == nil || *clientID == "" {
		return nil, nil // OAuth is optional
	}

	if clientSecret == nil || *clientSecret == "" {
		return nil, errors.New("invalid oauth config: client secret is empty")
	}
	if redirectUrl == nil || *redirectUrl == "" {
		return nil, errors.New("invalid oauth config: redirect URL is empty")
	}
	if authUrl == nil || *authUrl == "" {
		return nil, errors.New("invalid oauth config: auth URL is empty")
	}
	if tokenUrl == nil || *tokenUrl == "" {
		return nil, errors.New("invalid oauth config: token URL is empty")
	}

	return &oauth2.Config{
		ClientID:     *clientID,
		ClientSecret: *clientSecret,
		RedirectURL:  *redirectUrl,
		Scopes:       scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:  *authUrl,
			TokenURL: *tokenUrl,
		},
	}, nil
}

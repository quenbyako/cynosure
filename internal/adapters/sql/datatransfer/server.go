package datatransfer

import (
	"net/url"
	"time"

	db "github.com/quenbyako/cynosure/contrib/db/gen/go"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/ids"
	"golang.org/x/oauth2"
)

// ServerInfoFromDB converts database row to domain ServerInfo.
// This is pure type conversion - NO database queries.
func ServerInfoFromDB(row db.GetServerInfoRow) (*ports.ServerInfo, error) {
	// Parse URL
	sseLink, err := url.Parse(row.Url)
	if err != nil {
		return nil, err
	}

	// Build OAuth2 config from database row
	var authConfig *oauth2.Config
	if row.ClientID != nil && *row.ClientID != "" {
		redirectURL := "http://localhost" // Default fallback
		if row.RedirectUrl != nil && *row.RedirectUrl != "" {
			redirectURL = *row.RedirectUrl
		}

		authConfig = &oauth2.Config{
			ClientID:     *row.ClientID,
			ClientSecret: *row.ClientSecret,
			RedirectURL:  redirectURL,
			Scopes:       row.Scopes,
			Endpoint: oauth2.Endpoint{
				AuthURL:  *row.AuthUrl,
				TokenURL: *row.TokenUrl,
			},
		}
	}

	// Parse expiration
	var configExpiration time.Time
	if row.Expiration.Valid {
		configExpiration = row.Expiration.Time
	}

	return &ports.ServerInfo{
		SSELink:          sseLink,
		AuthConfig:       authConfig,
		ConfigExpiration: configExpiration,
	}, nil
}

// ServerInfoListFromDB converts multiple database rows to domain ServerInfo map.
// This is pure type conversion - NO database queries.
func ServerInfoListFromDB(rows []db.ListServersRow) (map[ids.ServerID]*ports.ServerInfo, error) {
	result := make(map[ids.ServerID]*ports.ServerInfo, len(rows))

	for _, row := range rows {
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
		var authConfig *oauth2.Config
		if row.ClientID != nil && *row.ClientID != "" {
			redirectURL := "http://localhost" // Default fallback
			if row.RedirectUrl != nil && *row.RedirectUrl != "" {
				redirectURL = *row.RedirectUrl
			}

			authConfig = &oauth2.Config{
				ClientID:     *row.ClientID,
				ClientSecret: *row.ClientSecret,
				RedirectURL:  redirectURL,
				Scopes:       row.Scopes,
				Endpoint: oauth2.Endpoint{
					AuthURL:  *row.AuthUrl,
					TokenURL: *row.TokenUrl,
				},
			}
		}

		// Parse expiration
		var configExpiration time.Time
		if row.Expiration.Valid {
			configExpiration = row.Expiration.Time
		}

		result[serverID] = &ports.ServerInfo{
			SSELink:          sseLink,
			AuthConfig:       authConfig,
			ConfigExpiration: configExpiration,
		}
	}

	return result, nil
}

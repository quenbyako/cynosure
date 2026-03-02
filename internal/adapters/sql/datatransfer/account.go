package datatransfer

import (
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/quenbyako/cynosure/contrib/db/gen/go"
	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

// AccountFromGetAccountRow converts a GetAccount row (with embedded token fields from LEFT JOIN)
// to a domain Account entity.
func AccountFromGetAccountRow(row db.GetAccountRow) (*entities.Account, error) {
	usrID, err := ids.NewUserID(row.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user id: %w", err)
	}

	srvID, err := ids.NewServerID(row.ServerID)
	if err != nil {
		return nil, fmt.Errorf("invalid server id: %w", err)
	}

	accountID, err := ids.NewAccountID(usrID, srvID, row.ID)
	if err != nil {
		return nil, fmt.Errorf("reconstructing account ID: %w", err)
	}

	return buildAccount(
		accountID,
		row.Name,
		row.Description,
		row.AccessToken,
		row.Type,
		row.RefreshToken,
		row.Expiry,
	)
}

// AccountFromGetAccountsBatchRow converts a GetAccountsBatch row
// (with embedded token fields from LEFT JOIN) to a domain Account entity.
func AccountFromGetAccountsBatchRow(row db.GetAccountsBatchRow) (*entities.Account, error) {
	usrID, err := ids.NewUserID(row.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user id: %w", err)
	}

	srvID, err := ids.NewServerID(row.ServerID)
	if err != nil {
		return nil, fmt.Errorf("invalid server id: %w", err)
	}

	accountID, err := ids.NewAccountID(usrID, srvID, row.ID)
	if err != nil {
		return nil, fmt.Errorf("reconstructing account ID: %w", err)
	}

	return buildAccount(
		accountID,
		row.Name,
		row.Description,
		row.AccessToken,
		row.Type,
		row.RefreshToken,
		row.Expiry,
	)
}

// buildAccount is a helper that constructs an Account entity from common fields
func buildAccount(
	accountID ids.AccountID,
	name string,
	description string,
	accessToken *string,
	tokenType *string,
	refreshToken *string,
	expiry pgtype.Timestamptz,
) (
	*entities.Account,
	error,
) {
	var token *oauth2.Token
	// Apply token if exists - token fields are optional from LEFT JOIN
	if accessToken != nil && *accessToken != "" {
		token = &oauth2.Token{
			AccessToken:  *accessToken,
			TokenType:    deref(tokenType),
			RefreshToken: deref(refreshToken),
		}
		if expiry.Valid {
			token.Expiry = expiry.Time
		}
	}

	acc, err := entities.NewAccount(
		accountID,
		name,
		description,
		entities.WithAuthToken(token),
	)
	if err != nil {
		return nil, fmt.Errorf("creating account entity: %w", err)
	}

	return acc, nil
}

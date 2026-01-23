package datatransfer

import (
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/quenbyako/cynosure/contrib/db/gen/go"
	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/tools"
)

// AccountFromGetAccountRow converts a GetAccount row (with embedded token fields from LEFT JOIN)
// and its associated tools to a domain Account entity.
func AccountFromGetAccountRow(row db.GetAccountRow, toolRows []db.AgentsMcpTool) (*entities.Account, error) {
	usrID, err := ids.NewUserID(row.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user id: %w", err)
	}

	srvID, err := ids.NewServerID(row.Server)
	if err != nil {
		return nil, fmt.Errorf("invalid server id: %w", err)
	}

	accountID, err := ids.NewAccountID(usrID, srvID, row.ID)
	if err != nil {
		return nil, fmt.Errorf("reconstructing account ID: %w", err)
	}

	return buildAccount(accountID, row.Name, row.Description, toolRows, row.AccessToken, row.Type, row.RefreshToken, row.Expiry)
}

// AccountFromGetAccountsBatchRow converts a GetAccountsBatch row (with embedded token fields from LEFT JOIN)
// and its associated tools to a domain Account entity.
func AccountFromGetAccountsBatchRow(row db.GetAccountsBatchRow, toolRows []db.AgentsMcpTool) (*entities.Account, error) {
	usrID, err := ids.NewUserID(row.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user id: %w", err)
	}

	srvID, err := ids.NewServerID(row.Server)
	if err != nil {
		return nil, fmt.Errorf("invalid server id: %w", err)
	}

	accountID, err := ids.NewAccountID(usrID, srvID, row.ID)
	if err != nil {
		return nil, fmt.Errorf("reconstructing account ID: %w", err)
	}

	return buildAccount(accountID, row.Name, row.Description, toolRows, row.AccessToken, row.Type, row.RefreshToken, row.Expiry)
}

// buildAccount is a helper that constructs an Account entity from common fields
func buildAccount(accountID ids.AccountID, name, description string, toolRows []db.AgentsMcpTool,
	accessToken *string, tokenType *string, refreshToken *string, expiry pgtype.Timestamp) (*entities.Account, error) {

	accTools := make([]tools.ToolInfo, len(toolRows))
	for i, tr := range toolRows {
		t, err := tools.NewToolInfo(
			tr.Name,
			"", // Description not in DB schema currently!
			tr.Input,
			tr.Output,
		)
		if err != nil {
			return nil, fmt.Errorf("reconstructing tool info %s: %w", tr.Name, err)
		}
		accTools[i] = t
	}

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
		accTools,
		entities.WithAuthToken(token),
	)
	if err != nil {
		return nil, fmt.Errorf("creating account entity: %w", err)
	}

	return acc, nil
}

// deref safely dereferences a pointer to string, returning empty string if nil
func deref[T any](s *T) T {
	if s == nil {
		var zero T
		return zero
	}
	return *s
}

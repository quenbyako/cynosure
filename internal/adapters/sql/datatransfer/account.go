package datatransfer

import (
	"fmt"

	db "github.com/quenbyako/cynosure/contrib/db/gen/go"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/tools"
	"golang.org/x/oauth2"
)

// AccountFromDBWithTools converts a database account record and its associated tools
// to a domain Account entity. Pure data transfer with no I/O or business logic.
func AccountFromDBWithTools(row db.AgentsMcpAccount, toolRows []db.AgentsMcpTool) (*entities.Account, error) {
	usrID, err := ids.NewUserID(row.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user id: %w", err)
	}

	srvID, err := ids.NewServerID(row.Server)
	if err != nil {
		return nil, fmt.Errorf("invalid server id: %w", err)
	}

	accountID, err := ids.NewAccountID(
		usrID,
		srvID,
		row.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("reconstructing account ID: %w", err)
	}

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
	// Apply token if exists (omitted for now as we deal with NULLs)
	if row.Token.Valid {
		token = &oauth2.Token{}
	}

	acc, err := entities.NewAccount(
		accountID,
		row.Name,
		row.Description,
		accTools,
		entities.WithAuthToken(token),
	)
	if err != nil {
		// NewAccount returns (*Account, error)
		return nil, fmt.Errorf("creating account entity: %w", err)
	}

	return acc, nil
}

package ory

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/identitymanager"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

// LookupUser searches for an identity by external identifier.
func (a *Client) LookupUser(ctx context.Context, externalID string) (ids.UserID, error) {
	resp, err := a.api.ListIdentitiesWithResponse(ctx)
	if err != nil {
		return ids.UserID{}, fmt.Errorf("performing request: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return ids.UserID{}, fmt.Errorf("ory error (status: %d): %s", resp.StatusCode(), string(resp.Body))
	}

	if resp.JSON200 == nil {
		return ids.UserID{}, identitymanager.ErrNotFound
	}

	for _, identity := range *resp.JSON200 {
		tgID, ok := identity.Traits["telegram_id"]
		if !ok {
			continue
		}

		var currentID string
		switch v := tgID.(type) {
		case float64:
			currentID = strconv.FormatFloat(v, 'f', -1, 64)
		case string:
			currentID = v
		case int:
			currentID = strconv.Itoa(v)
		case int64:
			currentID = strconv.FormatInt(v, 10)
		}

		if currentID == externalID {
			userID, err := ids.NewUserIDFromString(identity.Id.String())
			if err != nil {
				return ids.UserID{}, fmt.Errorf("parsing user id from ory: %w", err)
			}
			return userID, nil
		}
	}

	return ids.UserID{}, identitymanager.ErrNotFound
}

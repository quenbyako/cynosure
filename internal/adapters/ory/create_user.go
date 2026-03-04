package ory

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/quenbyako/cynosure/contrib/ory-openapi/gen/go/ory"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

const registredIdentitySchema = "5d0946b0f4e2e44a9bb8350f56493fd679fc88927e677b853514aec85046e9718fa956647bb0a035f9465d826591d7dcd330b68d64a655733f7617770083a95c"

// CreateUser creates a new identity in Ory with the given traits.
func (a *Client) CreateUser(ctx context.Context, externalID, username, firstName, lastName string) (ids.UserID, error) {
	idInt, err := strconv.ParseInt(externalID, 10, 64)
	if err != nil {
		return ids.UserID{}, fmt.Errorf("parsing external id: %w", err)
	}

	state := "active"
	resp, err := a.api.CreateIdentityWithResponse(ctx, ory.CreateIdentity{
		SchemaId: registredIdentitySchema,
		State:    &state,
		Traits: map[string]any{
			"telegram_id": idInt,
			"username":    username,
			"first_name":  firstName,
			"last_name":   lastName,
		},
	})
	if err != nil {
		return ids.UserID{}, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode() != http.StatusCreated {
		return ids.UserID{}, fmt.Errorf("ory error (status: %d): %s", resp.StatusCode(), string(resp.Body))
	}

	if resp.JSON201 == nil {
		return ids.UserID{}, fmt.Errorf("invalid response from ory: missing identity")
	}

	userID, err := ids.NewUserIDFromString(resp.JSON201.Id.String())
	if err != nil {
		return ids.UserID{}, fmt.Errorf("parsing created user id: %w", err)
	}

	return userID, nil
}

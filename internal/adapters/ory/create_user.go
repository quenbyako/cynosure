// Package ory provides an adapter for Ory Hydra and Kratos.
package ory

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/quenbyako/cynosure/contrib/ory-openapi/gen/go/ory"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

const (
	// registeredIdentitySchema is the schema ID for registered identities.
	registeredIdentitySchema = "5d0946b0f4e2e44a9bb8350f56493fd679fc88927e677b8535" +
		"14aec85046e9718fa956647bb0a035f9465d826591d7dcd330b68d64a655733f7617770083a95c"
)

// CreateUser creates a new user in Ory Kratos.
func (a *Adapter) CreateUser(
	ctx context.Context,
	externalID, username, firstName, lastName string,
) (ids.UserID, error) {
	ctx, ops := a.initiateAuth(ctx, "CreateUser")
	defer ops.End()

	telegramID, err := strconv.ParseInt(externalID, 10, 64)
	if err != nil {
		return ids.UserID{}, fmt.Errorf("invalid telegram id: %w", err)
	}

	return a.createUser(ctx, telegramID, username, firstName, lastName)
}

func (a *Adapter) createUser(
	ctx context.Context,
	telegramID int64, username, firstName, lastName string,
) (ids.UserID, error) {
	idBody := ory.CreateIdentity{
		SchemaId: registeredIdentitySchema,
		State:    ptr("active"),
		Traits: map[string]any{
			"telegram_id": telegramID,
			"username":    username,
			"first_name":  firstName,
			"last_name":   lastName,
		},
	}

	resp, err := a.api.CreateIdentityWithResponse(ctx, idBody)
	if err != nil {
		return ids.UserID{}, fmt.Errorf("create identity failed: %w", err)
	}

	return a.processCreateUserResponse(resp)
}

func (a *Adapter) processCreateUserResponse(
	resp *ory.CreateIdentityResponse,
) (ids.UserID, error) {
	if resp.StatusCode() != http.StatusCreated {
		return ids.UserID{}, fmt.Errorf("%w (status %d): %s",
			ErrInternal, resp.StatusCode(), string(resp.Body))
	}

	if resp.JSON201 == nil {
		return ids.UserID{}, fmt.Errorf("%w: missing identity in response",
			ErrUnexpectedResponse)
	}

	userID, err := ids.NewUserID(resp.JSON201.Id)
	if err != nil {
		return ids.UserID{}, fmt.Errorf("invalid user id in response: %w", err)
	}

	return userID, nil
}

func ptr[T any](v T) *T {
	return &v
}

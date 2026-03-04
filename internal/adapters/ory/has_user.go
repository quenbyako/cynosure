package ory

import (
	"context"
	"fmt"
	"net/http"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

// HasUser checks if identity exists in Ory.
func (a *Client) HasUser(ctx context.Context, id ids.UserID) (bool, error) {
	resp, err := a.api.GetIdentityWithResponse(ctx, id.ID().String())
	if err != nil {
		return false, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode() == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode() != http.StatusOK {
		return false, fmt.Errorf("ory error (status: %d): %s", resp.StatusCode(), string(resp.Body))
	}

	return true, nil
}

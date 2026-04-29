package ory

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/quenbyako/cynosure/contrib/ory-openapi/gen/go/ory"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/identitymanager"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

// LookupUser looks up a user in Ory Kratos by their external ID.
func (a *Adapter) LookupUser(ctx context.Context, externalID string) (ids.UserID, error) {
	resp, err := a.listIdentitiesByIdentifier(ctx, externalID)
	if err != nil {
		return ids.UserID{}, err
	}

	return a.processLookupResponse(resp)
}

func (a *Adapter) listIdentitiesByIdentifier(
	ctx context.Context,
	externalID string,
) (*ory.ListIdentitiesResponse, error) {
	resp, err := a.api.ListIdentitiesWithResponse(ctx,
		func(_ context.Context, req *http.Request) error {
			q := req.URL.Query()
			q.Set("credentials_identifier", externalID)
			req.URL.RawQuery = q.Encode()

			return nil
		})
	if err != nil {
		return nil, fmt.Errorf("listing identities: %w", err)
	}

	return resp, nil
}

func (a *Adapter) processLookupResponse(
	resp *ory.ListIdentitiesResponse,
) (ids.UserID, error) {
	if resp.StatusCode() == http.StatusTooManyRequests {
		return ids.UserID{}, identitymanager.ErrRateLimited
	}

	if resp.StatusCode() != http.StatusOK {
		return ids.UserID{}, fmt.Errorf("%w (status %d): %s",
			ErrInternal, resp.StatusCode(), formatErrorBody(resp.HTTPResponse, resp.Body))
	}

	if resp.JSON200 == nil || len(*resp.JSON200) == 0 {
		return ids.UserID{}, identitymanager.ErrNotFound
	}

	identities := *resp.JSON200

	uid, err := ids.NewUserID(identities[0].Id)
	if err != nil {
		return ids.UserID{}, fmt.Errorf("invalid user id from ory: %w", err)
	}

	return uid, nil
}

func formatErrorBody(resp *http.Response, body []byte) string {
	if len(body) == 0 {
		return "empty body"
	}

	contentType := ""
	if resp != nil {
		contentType = resp.Header.Get("Content-Type")
	}

	s := string(body)
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.TrimSpace(s)

	switch {
	case strings.Contains(contentType, "text/html"):
		if len(s) > 100 {
			return fmt.Sprintf("(html): %s...", s[:100])
		}

		return "(html): " + s
	case strings.Contains(contentType, "application/json"):
		if len(s) > 200 {
			return fmt.Sprintf("(json): %s...", s[:200])
		}

		return "(json): " + s
	default:
		if len(s) > 150 {
			return s[:150] + "..."
		}

		return s
	}
}

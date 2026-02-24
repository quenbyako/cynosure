// TODO: here we are using raw requests, instead of utilizing openapi schema for
// ory api. This is a tech debt for sure.
package ory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

const pkgName = "github.com/quenbyako/cynosure/internal/adapters/ory"

type Client struct {
	baseURL  string
	adminKey string

	trace trace.Tracer
}

var _ ports.IdentityManager = (*Client)(nil)
var _ ports.IdentityManagerFactory = (*Client)(nil)

type identityTraits struct {
	TelegramID int64  `json:"telegram_id"`
	Username   string `json:"username,omitempty"`
	FirstName  string `json:"first_name,omitempty"`
	LastName   string `json:"last_name,omitempty"`
}

type identity struct {
	ID       string          `json:"id"`
	Traits   json.RawMessage `json:"traits"`
	SchemaID string          `json:"schema_id"`
	State    string          `json:"state"`
}

type createIdentityBody struct {
	SchemaID string         `json:"schema_id"`
	Traits   identityTraits `json:"traits"`
	State    string         `json:"state"`
}

type newParams struct {
	trace trace.TracerProvider
}

type NewOption func(*newParams)

func WithTracerProvider(trace trace.TracerProvider) NewOption {
	return func(p *newParams) { p.trace = trace }
}

func New(endpoint *url.URL, adminKey string, opts ...NewOption) *Client {
	p := newParams{
		trace: noop.NewTracerProvider(),
	}

	for _, opt := range opts {
		opt(&p)
	}

	return &Client{
		baseURL:  endpoint.String(),
		adminKey: adminKey,
		trace:    p.trace.Tracer(pkgName),
	}
}

// IdentityManager implements ports.IdentityManagerFactory.
func (a *Client) IdentityManager() ports.IdentityManagerWrapped {
	return ports.WrapIdentityManager(a, ports.WithTrace(a.trace))
}

func (a *Client) request(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, a.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+a.adminKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return http.DefaultClient.Do(req)
}

// HasUser checks if identity exists in Ory.
func (a *Client) HasUser(ctx context.Context, id ids.UserID) (bool, error) {
	resp, err := a.request(ctx, "GET", "/admin/identities/"+id.ID().String(), nil)
	if err != nil {
		return false, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("ory error (status: %d): %s", resp.StatusCode, string(body))
	}

	return true, nil
}

// LookupUser searches for an identity by external identifier.
func (a *Client) LookupUser(ctx context.Context, externalID string) (ids.UserID, error) {
	ctx, span := a.trace.Start(ctx, "LookupUser")
	defer span.End()

	resp, err := a.request(ctx, "GET", "/admin/identities", nil)
	if err != nil {
		span.RecordError(err)
		return ids.UserID{}, fmt.Errorf("performing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		err := fmt.Errorf("ory error (status: %d): %s", resp.StatusCode, string(body))
		span.RecordError(err)
		return ids.UserID{}, err
	}

	var identities []identity
	if err := json.NewDecoder(resp.Body).Decode(&identities); err != nil {
		return ids.UserID{}, fmt.Errorf("decoding identities: %w", err)
	}

	for _, identity := range identities {
		var traits map[string]interface{}
		if err := json.Unmarshal(identity.Traits, &traits); err != nil {
			continue
		}

		tgID, ok := traits["telegram_id"]
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
			userID, err := ids.NewUserIDFromString(identity.ID)
			if err != nil {
				return ids.UserID{}, fmt.Errorf("parsing user id from ory: %w", err)
			}
			return userID, nil
		}
	}

	return ids.UserID{}, ports.ErrNotFound
}

const registredIdentitySchema = "5d0946b0f4e2e44a9bb8350f56493fd679fc88927e677b853514aec85046e9718fa956647bb0a035f9465d826591d7dcd330b68d64a655733f7617770083a95c"

// CreateUser creates a new identity in Ory with the given traits.
func (a *Client) CreateUser(ctx context.Context, externalID, username, firstName, lastName string) (ids.UserID, error) {
	idInt, err := strconv.ParseInt(externalID, 10, 64)
	if err != nil {
		return ids.UserID{}, fmt.Errorf("parsing external id: %w", err)
	}

	traits := identityTraits{
		TelegramID: idInt,
		Username:   username,
		FirstName:  firstName,
		LastName:   lastName,
	}

	bodyObj := createIdentityBody{
		SchemaID: registredIdentitySchema,
		Traits:   traits,
		State:    "active",
	}

	bodyBytes, err := json.Marshal(bodyObj)
	if err != nil {
		return ids.UserID{}, fmt.Errorf("marshaling request body: %w", err)
	}

	resp, err := a.request(ctx, "POST", "/admin/identities", bytes.NewReader(bodyBytes))
	if err != nil {
		return ids.UserID{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return ids.UserID{}, fmt.Errorf("ory error (status: %d): %s", resp.StatusCode, string(body))
	}

	var iden identity
	if err := json.NewDecoder(resp.Body).Decode(&iden); err != nil {
		return ids.UserID{}, fmt.Errorf("decoding response: %w", err)
	}

	userID, err := ids.NewUserIDFromString(iden.ID)
	if err != nil {
		return ids.UserID{}, fmt.Errorf("parsing created user id: %w", err)
	}

	return userID, nil
}

// SaveUserMapping is a no-op since Ory identity already contains the mapping in its traits.
func (a *Client) SaveUserMapping(ctx context.Context, id ids.UserID, provider, externalID string) error {
	return nil
}

package ory

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/quenbyako/cynosure/contrib/ory-openapi/gen/go/ory"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/identitymanager"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"golang.org/x/oauth2"
)

// IssueToken issues a new OAuth2 token for the given user by programmatically
// accepting login and consent requests in Ory Hydra.
func (a *Client) IssueToken(ctx context.Context, id ids.UserID) (*oauth2.Token, error) {
	for i := 0; i < 3; i++ {
		token, err := a.issueToken(ctx, id)
		if err == nil {
			return token, nil
		}
		if !errors.Is(err, identitymanager.ErrRateLimited) {
			return nil, err
		}

		// Wait before retry
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second * time.Duration(i+1)):
		}
	}
	return nil, identitymanager.ErrRateLimited
}

func (a *Client) issueToken(ctx context.Context, id ids.UserID) (*oauth2.Token, error) {
	// 1. Check if user exists
	exists, err := a.HasUser(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("checking user existence: %w", err)
	}
	if !exists {
		return nil, identitymanager.ErrNotFound
	}

	// 2. Start OAuth2 flow
	state := "state-longer-than-eight-characters"
	authURL := a.config.AuthCodeURL(state)

	jar, _ := cookiejar.New(nil)
	httpClient := &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// 3. Initiate Auth
	challenge, err := a.initiateAuth(ctx, httpClient, authURL)
	if err != nil {
		return nil, err
	}

	var consentChallenge string
	if strings.HasPrefix(challenge, "consent:") {
		consentChallenge = strings.TrimPrefix(challenge, "consent:")
	} else {
		// 4. Accept Login
		loginRedirectTo, err := a.acceptLogin(ctx, challenge, id.ID().String())
		if err != nil {
			return nil, err
		}

		// 5. Follow to Consent
		consentChallenge, err = a.followToConsent(ctx, httpClient, loginRedirectTo)
		if err != nil {
			return nil, err
		}
	}

	// 6. Accept Consent
	consentRedirectTo, err := a.acceptConsent(ctx, consentChallenge)
	if err != nil {
		return nil, err
	}

	// 7. Get the code
	code, err := a.exchangeCode(ctx, httpClient, consentRedirectTo)
	if err != nil {
		return nil, err
	}

	// 8. Exchange
	return a.config.Exchange(ctx, code)
}

func (a *Client) initiateAuth(ctx context.Context, client *http.Client, authURL string) (string, error) {
	ctx, span := a.obs.initiateAuth(ctx, "initiateAuth")
	defer span.end()

	currURL := authURL
	for i := 0; i < 10; i++ {
		resp, err := client.Get(currURL)
		if err != nil {
			span.recordError(err)
			return "", fmt.Errorf("initiating auth flow at %s: %w", currURL, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusTooManyRequests {
			return "", identitymanager.ErrRateLimited
		}

		// Check query of the current request URL (if we were redirected and landed)
		if c := resp.Request.URL.Query().Get("login_challenge"); c != "" {
			return c, nil
		}
		if c := resp.Request.URL.Query().Get("consent_challenge"); c != "" {
			return "consent:" + c, nil
		}

		// If it's a redirect, check the Location header directly too
		if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusSeeOther {
			location, err := resp.Location()
			if err == nil {
				if c := location.Query().Get("login_challenge"); c != "" {
					return c, nil
				}
				if c := location.Query().Get("consent_challenge"); c != "" {
					return "consent:" + c, nil
				}
				currURL = location.String()
				continue
			}
		}

		return "", fmt.Errorf("login challenge not found at %s (status %d)", resp.Request.URL.String(), resp.StatusCode)
	}
	return "", fmt.Errorf("too many redirects during auth initiation")
}

func (a *Client) acceptLogin(ctx context.Context, challenge, subject string) (string, error) {
	ctx, span := a.obs.step(ctx, "acceptLogin")
	defer span.end()

	resp, err := a.api.AcceptLoginRequestWithResponse(ctx, ory.AcceptOAuth2LoginRequest{
		Subject: subject,
	}, func(ctx context.Context, req *http.Request) error {
		q := req.URL.Query()
		q.Set("login_challenge", challenge)
		req.URL.RawQuery = q.Encode()
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("accepting login: %w", err)
	}

	if resp.StatusCode() == http.StatusTooManyRequests {
		return "", identitymanager.ErrRateLimited
	}

	if resp.StatusCode() != http.StatusOK {
		return "", fmt.Errorf("failed to accept login (status %d): %s", resp.StatusCode(), string(resp.Body))
	}

	if resp.JSON200 == nil || resp.JSON200.RedirectTo == nil {
		return "", fmt.Errorf("invalid response from ory: missing redirect_to")
	}

	return *resp.JSON200.RedirectTo, nil
}
func (a *Client) followToConsent(ctx context.Context, client *http.Client, redirectTo string) (string, error) {
	ctx, span := a.obs.step(ctx, "followToConsent")
	defer span.end()

	currURL := redirectTo
	for i := 0; i < 10; i++ {
		resp, err := client.Get(currURL)
		if err != nil {
			span.recordError(err)
			return "", fmt.Errorf("following to consent at %s: %w", currURL, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusTooManyRequests {
			return "", identitymanager.ErrRateLimited
		}

		if c := resp.Request.URL.Query().Get("consent_challenge"); c != "" {
			return c, nil
		}

		if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusSeeOther {
			location, err := resp.Location()
			if err == nil {
				if c := location.Query().Get("consent_challenge"); c != "" {
					return c, nil
				}
				currURL = location.String()
				continue
			}
		}

		return "", fmt.Errorf("consent challenge not found at %s (status %d)", resp.Request.URL.String(), resp.StatusCode)
	}
	return "", fmt.Errorf("too many redirects during consent following")
}

func (a *Client) acceptConsent(ctx context.Context, challenge string) (string, error) {
	ctx, span := a.obs.step(ctx, "acceptConsent")
	defer span.end()

	resp, err := a.api.AcceptConsentRequestWithResponse(ctx, ory.AcceptOAuth2ConsentRequest{
		GrantScope: &a.config.Scopes,
	}, func(ctx context.Context, req *http.Request) error {
		q := req.URL.Query()
		q.Set("consent_challenge", challenge)
		req.URL.RawQuery = q.Encode()
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("accepting consent: %w", err)
	}

	if resp.StatusCode() == http.StatusTooManyRequests {
		return "", identitymanager.ErrRateLimited
	}

	if resp.StatusCode() != http.StatusOK {
		return "", fmt.Errorf("failed to accept consent (status %d): %s", resp.StatusCode(), string(resp.Body))
	}

	if resp.JSON200 == nil || resp.JSON200.RedirectTo == nil {
		return "", fmt.Errorf("invalid response from ory: missing redirect_to")
	}

	return *resp.JSON200.RedirectTo, nil
}

func (a *Client) exchangeCode(ctx context.Context, client *http.Client, redirectTo string) (string, error) {
	ctx, span := a.obs.step(ctx, "exchangeCode")
	defer span.end()

	currURL := redirectTo
	for i := 0; i < 10; i++ {
		resp, err := client.Get(currURL)
		if err != nil {
			span.recordError(err)
			return "", fmt.Errorf("following to exchange at %s: %w", currURL, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusTooManyRequests {
			return "", identitymanager.ErrRateLimited
		}

		if c := resp.Header.Get("Location"); c != "" {
			u, _ := url.Parse(c)
			if code := u.Query().Get("code"); code != "" {
				return code, nil
			}
			currURL = c
			continue
		}

		if resp.StatusCode == http.StatusOK {
			if code := resp.Request.URL.Query().Get("code"); code != "" {
				return code, nil
			}
		}

		return "", fmt.Errorf("auth code not found at %s (status %d)", resp.Request.URL.String(), resp.StatusCode)
	}
	return "", fmt.Errorf("too many redirects during code exchange")
}

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
	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/identitymanager"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

const (
	maxRedirects = 10
	retryCount   = 3
	stateValue   = "state-longer-than-eight-characters"
)

// IssueToken issues a new OAuth2 token for the given user.
func (a *Client) IssueToken(ctx context.Context, id ids.UserID) (*oauth2.Token, error) {
	for i := range retryCount {
		token, err := a.issueToken(ctx, id)
		if err == nil {
			return token, nil
		}

		if !errors.Is(err, identitymanager.ErrRateLimited) {
			return nil, err
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled during token issuance: %w", ctx.Err())
		case <-time.After(time.Second * time.Duration(i+1)):
		}
	}

	return nil, identitymanager.ErrRateLimited
}

func (a *Client) issueToken(ctx context.Context, id ids.UserID) (*oauth2.Token, error) {
	exists, err := a.HasUser(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("checking user existence: %w", err)
	}

	if !exists {
		return nil, identitymanager.ErrNotFound
	}

	httpClient, err := a.newHTTPClient()
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	challenge, err := a.getConsentChallenge(ctx, httpClient, id)
	if err != nil {
		return nil, err
	}

	redirectTo, err := a.acceptConsent(ctx, challenge)
	if err != nil {
		return nil, err
	}

	code, err := a.exchangeCode(ctx, httpClient, redirectTo)
	if err != nil {
		return nil, err
	}

	return a.exchangeToken(ctx, code)
}

func (a *Client) newHTTPClient() (*http.Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("creating cookie jar: %w", err)
	}

	return &http.Client{
		Jar: jar,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: http.DefaultTransport,
		Timeout:   0,
	}, nil
}

func (a *Client) exchangeToken(ctx context.Context, code string) (*oauth2.Token, error) {
	token, err := a.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchanging code for token: %w", err)
	}

	return token, nil
}

func (a *Client) getConsentChallenge(
	ctx context.Context,
	httpClient *http.Client,
	id ids.UserID,
) (string, error) {
	authURL := a.config.AuthCodeURL(stateValue)

	challenge, err := a.doInitiateAuth(ctx, httpClient, authURL)
	if err != nil {
		return "", err
	}

	if strings.HasPrefix(challenge, "consent:") {
		return strings.TrimPrefix(challenge, "consent:"), nil
	}

	loginRedirectTo, err := a.acceptLogin(ctx, challenge, id.ID().String())
	if err != nil {
		return "", err
	}

	return a.followToConsent(ctx, httpClient, loginRedirectTo)
}

func (a *Client) doInitiateAuth(
	ctx context.Context,
	httpClient *http.Client,
	authURL string,
) (string, error) {
	innerCtx, ops := a.obs.initiateAuth(ctx, "initiateAuth")
	defer ops.End()

	currURL := authURL

	for range maxRedirects {
		challenge, location, err := a.doAuthStep(innerCtx, ops, httpClient, currURL)
		if err != nil {
			return "", err
		}

		if challenge != "" {
			return challenge, nil
		}

		currURL = location
	}

	return "", ErrTooManyRedirects
}

type authStepRes struct {
	challenge string
	location  string
}

func (a *Client) doAuthStep(
	ctx context.Context,
	ops span,
	httpClient *http.Client,
	currURL string,
) (challenge, location string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, currURL, http.NoBody)
	if err != nil {
		return "", "", fmt.Errorf("creating auth request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		ops.RecordError(err)

		return "", "", fmt.Errorf("initiating auth flow at %s: %w", currURL, err)
	}

	//nolint:errcheck
	defer func() { _ = resp.Body.Close() }()

	res, err := a.parseAuthResponse(resp)
	if err != nil {
		return "", "", err
	}

	return res.challenge, res.location, nil
}

func (a *Client) parseAuthResponse(resp *http.Response) (*authStepRes, error) {
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, ErrRateLimited
	}

	if chal, ok := a.checkURLForChallenge(resp.Request.URL); ok {
		return &authStepRes{challenge: chal, location: ""}, nil
	}

	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		return nil, fmt.Errorf("%w (status %d)", ErrUnexpectedResponse, resp.StatusCode)
	}

	loc, err := resp.Location()
	if err != nil {
		return nil, fmt.Errorf("location missing at %s: %w", resp.Request.URL.String(), err)
	}

	if chal, ok := a.checkURLForChallenge(loc); ok {
		return &authStepRes{challenge: chal, location: ""}, nil
	}

	return &authStepRes{challenge: "", location: loc.String()}, nil
}

func (a *Client) checkURLForChallenge(u *url.URL) (string, bool) {
	if challenge := u.Query().Get("login_challenge"); challenge != "" {
		return challenge, true
	}

	if challenge := u.Query().Get("consent_challenge"); challenge != "" {
		return "consent:" + challenge, true
	}

	return "", false
}

func (a *Client) acceptLogin(ctx context.Context, challenge, subject string) (string, error) {
	_, ops := a.obs.step(ctx, "acceptLogin")
	defer ops.End()

	resp, err := a.api.AcceptLoginRequestWithResponse(ctx, ory.AcceptOAuth2LoginRequest{
		Remember:    nil,
		RememberFor: nil,
		Subject:     subject,
	}, func(_ context.Context, req *http.Request) error {
		q := req.URL.Query()
		q.Set("login_challenge", challenge)
		req.URL.RawQuery = q.Encode()

		return nil
	})
	if err != nil {
		return "", fmt.Errorf("accepting login: %w", err)
	}

	return a.processAcceptRedirect(resp.StatusCode(), resp.Body, resp.JSON200)
}

func (a *Client) processAcceptRedirect(
	code int,
	body []byte,
	data *ory.OAuth2RedirectTo,
) (string, error) {
	if code == http.StatusTooManyRequests {
		return "", ErrRateLimited
	}

	if code != http.StatusOK {
		return "", fmt.Errorf("%w (status %d): %s",
			ErrInternal, code, string(body))
	}

	if data == nil || data.RedirectTo == nil {
		return "", ErrRedirectMissing
	}

	return *data.RedirectTo, nil
}

func (a *Client) followToConsent(
	ctx context.Context,
	httpClient *http.Client,
	redirectTo string,
) (string, error) {
	innerCtx, ops := a.obs.step(ctx, "followToConsent")
	defer ops.End()

	currURL := redirectTo

	for range maxRedirects {
		chal, loc, err := a.doRedirectStep(innerCtx, ops, httpClient,
			currURL, a.processConsentResponse)
		if err != nil {
			return "", err
		}

		if chal != "" {
			return chal, nil
		}

		currURL = loc
	}

	return "", ErrTooManyRedirects
}

type redirectProc func(resp *http.Response) (challenge, location string, ok bool)

func (a *Client) doRedirectStep(
	ctx context.Context,
	ops span,
	httpClient *http.Client,
	currURL string,
	proc redirectProc,
) (challenge, location string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, currURL, http.NoBody)
	if err != nil {
		return "", "", fmt.Errorf("creating redirect request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		ops.RecordError(err)

		return "", "", fmt.Errorf("following redirect at %s: %w", currURL, err)
	}

	defer func() { _ = resp.Body.Close() }() //nolint:errcheck

	challenge, location, ok := proc(resp)
	if ok {
		return challenge, "", nil
	}

	if location == "" {
		return "", "", fmt.Errorf("%w (redirect location missing)", ErrUnexpectedResponse)
	}

	return "", location, nil
}

func (a *Client) processConsentResponse(resp *http.Response) (challenge, location string, ok bool) {
	if chal, ok := a.extractConsentChallenge(resp); ok {
		return chal, "", true
	}

	loc, err := resp.Location()
	if err != nil {
		return "", "", false
	}

	return "", loc.String(), false
}

func (a *Client) extractConsentChallenge(resp *http.Response) (string, bool) {
	if resp.StatusCode == http.StatusTooManyRequests {
		return "", false
	}

	if challenge := resp.Request.URL.Query().Get("consent_challenge"); challenge != "" {
		return challenge, true
	}

	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		return "", false
	}

	loc, err := resp.Location()
	if err != nil {
		return "", false
	}

	if chal, ok := loc.Query()["consent_challenge"]; ok && len(chal) > 0 {
		return chal[0], true
	}

	return "", false
}

func (a *Client) acceptConsent(ctx context.Context, challenge string) (string, error) {
	_, ops := a.obs.step(ctx, "acceptConsent")
	defer ops.End()

	resp, err := a.api.AcceptConsentRequestWithResponse(ctx, ory.AcceptOAuth2ConsentRequest{
		GrantScope:  &a.config.Scopes,
		Remember:    nil,
		RememberFor: nil,
	}, func(_ context.Context, req *http.Request) error {
		q := req.URL.Query()
		q.Set("consent_challenge", challenge)
		req.URL.RawQuery = q.Encode()

		return nil
	})
	if err != nil {
		return "", fmt.Errorf("accepting consent: %w", err)
	}

	return a.processAcceptRedirect(resp.StatusCode(), resp.Body, resp.JSON200)
}

func (a *Client) exchangeCode(
	ctx context.Context,
	httpClient *http.Client,
	redirectTo string,
) (string, error) {
	innerCtx, ops := a.obs.step(ctx, "exchangeCode")
	defer ops.End()

	currURL := redirectTo

	for range maxRedirects {
		code, loc, err := a.doRedirectStep(innerCtx, ops, httpClient,
			currURL, a.processExchangeResponse)
		if err != nil {
			return "", err
		}

		if code != "" {
			return code, nil
		}

		currURL = loc
	}

	return "", ErrTooManyRedirects
}

func (a *Client) processExchangeResponse(resp *http.Response) (code, location string, ok bool) {
	if c, ok := a.extractCode(resp); ok {
		return c, "", true
	}

	loc := resp.Header.Get("Location")

	return "", loc, false
}

func (a *Client) extractCode(resp *http.Response) (string, bool) {
	location := resp.Header.Get("Location")
	if location != "" {
		u, err := url.Parse(location)
		if err == nil && u.Query().Get("code") != "" {
			return u.Query().Get("code"), true
		}
	}

	if resp.StatusCode == http.StatusOK {
		if code := resp.Request.URL.Query().Get("code"); code != "" {
			return code, true
		}
	}

	return "", false
}

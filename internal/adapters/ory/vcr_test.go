package ory_test

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/dnaeon/go-vcr.v4/pkg/cassette"
	"gopkg.in/dnaeon/go-vcr.v4/pkg/recorder"

	"github.com/quenbyako/cynosure/internal/adapters/ory"
	"github.com/quenbyako/cynosure/internal/vcrtest"
)

//go:embed testdata/*
var testdata embed.FS

const (
	redactedURL = "https://dummy.internal"
)

type oryCreds struct {
	endpoint     *url.URL
	fakeURL      *url.URL
	adminKey     string
	clientID     string
	clientSecret string
}

func getOryCredentials(t *testing.T) oryCreds {
	t.Helper()

	endpoint, err := url.Parse(vcrtest.OryEndpoint(t))
	require.NoError(t, err)

	fakeURL, err := url.Parse(redactedURL)
	require.NoError(t, err)

	return oryCreds{
		endpoint:     endpoint,
		fakeURL:      fakeURL,
		adminKey:     vcrtest.OryAdminKey(t),
		clientID:     vcrtest.OryClientID(t),
		clientSecret: vcrtest.OryClientSecret(t),
	}
}

func newOryClient(creds oryCreds, rec *recorder.Recorder) (*ory.Adapter, error) {
	adapter, err := ory.New(creds.fakeURL, creds.adminKey,
		ory.WithHTTPClient(rec),
		ory.WithClientCredentials(creds.clientID, creds.clientSecret),
		ory.WithScopes("mcp:read", "mcp:write", "offline_access"),
		ory.WithRedirectURL("http://localhost:5001"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating client: %w", err)
	}

	return adapter, nil
}

func vcrAdapter(
	t *testing.T, name string,
) (adapter *ory.Adapter, telegramUserID int64, stop func() error) {
	t.Helper()

	creds := getOryCredentials(t)

	const userID = 987654321

	opts := buildRecorderOptions(t, creds)

	rec := vcrtest.New(
		t, testdata,
		strings.TrimSuffix(name, ".yaml"),
		opts...,
	)

	gem, err := newOryClient(creds, rec)
	require.NoError(t, err)

	return gem, userID, rec.Stop
}

func buildRecorderOptions(
	t *testing.T, creds oryCreds,
) []recorder.Option {
	t.Helper()

	return []recorder.Option{
		recorder.WithMatcher(cassette.NewDefaultMatcher(
			cassette.WithIgnoreAuthorization(),
			cassette.WithIgnoreHeaders("Set-Cookie", "Cookie"),
		)),
		recorder.WithHook(func(call *cassette.Interaction) error {
			redactHeaders(call)
			redactRequestSecrets(call, creds.clientID, creds.clientSecret, creds.adminKey)
			redactResponseSecrets(call, creds.clientID, creds.clientSecret, creds.adminKey)

			return nil
		}, recorder.BeforeSaveHook),
		recorder.WithRealTransport(&domainReplacer{
			t:           t,
			originalURL: creds.endpoint,
			fakeURL:     creds.fakeURL,
			inner:       http.DefaultTransport,
		}),
	}
}

func redactHeaders(call *cassette.Interaction) {
	if call.Request.Headers.Get("Authorization") != "" {
		call.Request.Headers.Set("Authorization", "REDACTED")
	}

	if call.Request.Headers.Get("Cookie") != "" {
		call.Request.Headers.Set("Cookie", "REDACTED")
	}

	if call.Response.Headers.Get("Set-Cookie") != "" {
		call.Response.Headers.Set("Set-Cookie", "REDACTED")
	}
}

// reason why we use replacer transport instead of real one is that Ory API ties
// responses to specific domain, without accessibility to change DNS name, etc.
// We can't save original endpoint in cassettes for security reasons, so it's
// required to replace EVERY endpoint to original.
type domainReplacer struct {
	t           *testing.T
	inner       http.RoundTripper
	originalURL *url.URL
	fakeURL     *url.URL
}

var _ http.RoundTripper = (*domainReplacer)(nil)

func (d *domainReplacer) RoundTrip(req *http.Request) (*http.Response, error) {
	originalReq := req
	req = req.Clone(req.Context())
	d.rewriteRequest(req)

	resp, err := d.inner.RoundTrip(req)
	if err != nil {
		return nil, fmt.Errorf("round trip: %w", err)
	}

	resp.Request = originalReq
	d.rewriteResponse(resp)

	return resp, nil
}

func (d *domainReplacer) rewriteRequest(req *http.Request) {
	if req.URL.Scheme == d.fakeURL.Scheme && req.URL.Host == d.fakeURL.Host {
		req.URL.Scheme = d.originalURL.Scheme
		req.URL.Host = d.originalURL.Host
	}

	if req.Host == d.fakeURL.Host {
		req.Host = d.originalURL.Host
	}

	d.replaceHeaders(req.Header, d.fakeURL.Host, d.originalURL.Host)

	if req.Body != nil && req.Body != http.NoBody {
		body, err := io.ReadAll(req.Body)
		require.NoError(d.t, err)
		require.NoError(d.t, req.Body.Close())
		body = bytes.ReplaceAll(body, []byte(d.fakeURL.Host), []byte(d.originalURL.Host))
		req.Body = io.NopCloser(bytes.NewReader(body))
	}
}

func (d *domainReplacer) rewriteResponse(resp *http.Response) {
	if resp == nil {
		return
	}

	d.replaceHeaders(resp.Header, d.originalURL.Host, d.fakeURL.Host)

	if resp.Body != nil && resp.Body != http.NoBody {
		body, err := io.ReadAll(resp.Body)
		require.NoError(d.t, err)
		require.NoError(d.t, resp.Body.Close())
		body = bytes.ReplaceAll(body, []byte(d.originalURL.Host), []byte(d.fakeURL.Host))
		resp.Body = io.NopCloser(bytes.NewReader(body))
	}
}

func (d *domainReplacer) replaceHeaders(h http.Header, oldStr, newStr string) {
	for k, vals := range h {
		for idx, v := range vals {
			h[k][idx] = strings.ReplaceAll(v, oldStr, newStr)
		}
	}
}

func replaceSecrets(content, clientID, clientSecret, adminKey string) string {
	if clientID != "" {
		content = strings.ReplaceAll(content, clientID, "dummy-ory-client-id")
		content = strings.ReplaceAll(
			content, url.QueryEscape(clientID), "dummy-ory-client-id",
		)
	}

	if clientSecret != "" {
		content = strings.ReplaceAll(content, clientSecret, "dummy-ory-client-secret")
		content = strings.ReplaceAll(
			content, url.QueryEscape(clientSecret), "dummy-ory-client-secret",
		)
	}

	if adminKey != "" {
		content = strings.ReplaceAll(content, adminKey, "dummy-ory-admin-key")
		content = strings.ReplaceAll(
			content, url.QueryEscape(adminKey), "dummy-ory-admin-key",
		)
	}

	return content
}

func replaceHeadersSecrets(h http.Header, clientID, clientSecret, adminKey string) {
	for k, vals := range h {
		for idx, v := range vals {
			h[k][idx] = replaceSecrets(v, clientID, clientSecret, adminKey)
		}
	}
}

func replaceFormSecrets(form url.Values, clientID, clientSecret, adminKey string) {
	for k, vals := range form {
		for idx, v := range vals {
			form[k][idx] = replaceSecrets(v, clientID, clientSecret, adminKey)
		}
	}
}

func redactRequestSecrets(
	call *cassette.Interaction,
	clientID, clientSecret, adminKey string,
) {
	call.Request.URL = replaceSecrets(
		call.Request.URL, clientID, clientSecret, adminKey,
	)
	call.Request.Body = replaceSecrets(
		call.Request.Body, clientID, clientSecret, adminKey,
	)
	call.Request.RequestURI = replaceSecrets(
		call.Request.RequestURI, clientID, clientSecret, adminKey,
	)

	replaceHeadersSecrets(
		call.Request.Headers, clientID, clientSecret, adminKey,
	)
	replaceFormSecrets(
		call.Request.Form, clientID, clientSecret, adminKey,
	)
}

func redactJSONTokens(body string) string {
	var data map[string]any

	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return body
	}

	if _, ok := data["access_token"]; ok {
		data["access_token"] = "dummy-access-token"
	}

	if _, ok := data["refresh_token"]; ok {
		data["refresh_token"] = "dummy-refresh-token"
	}

	res, err := json.Marshal(data)
	if err != nil {
		return body
	}

	return string(res)
}

func redactResponseSecrets(
	call *cassette.Interaction,
	clientID, clientSecret, adminKey string,
) {
	body := replaceSecrets(
		call.Response.Body, clientID, clientSecret, adminKey,
	)

	call.Response.Body = redactJSONTokens(body)

	replaceHeadersSecrets(
		call.Response.Headers, clientID, clientSecret, adminKey,
	)
}

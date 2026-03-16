package testsuite

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	. "github.com/quenbyako/cynosure/internal/domains/cynosure/ports/oauthhandler"
)

// RunOAuthHandlerTests runs tests for the given adapter. These tests are predefined
// and REQUIRED to be used for ANY adapter implementation.
//
// After constructing test function, just run it through `run(t)` call,
// everything else will be handled for you. Calling this function through
// `t.Run("general", run)` is not very recommended, cause test logs will be too
// hard to read cause of big nesting.
func RunOAuthHandlerTests(
	a Port, opts ...OAuthHandlerTestSuiteOption,
) func(t *testing.T) {
	suite := &OAuthHandlerTestSuite{
		adapter: a,
		cleanup: nil,
	}
	for _, opt := range opts {
		opt(suite)
	}

	if err := suite.validate(); err != nil {
		panic(err)
	}

	return runSuite(suite)
}

type OAuthHandlerTestSuite struct {
	adapter Port

	cleanup func() error
}

var _ afterTest = (*OAuthHandlerTestSuite)(nil)

type OAuthHandlerTestSuiteOption func(*OAuthHandlerTestSuite)

func WithOAuthHandlerCleanup(f func() error) OAuthHandlerTestSuiteOption {
	return func(s *OAuthHandlerTestSuite) { s.cleanup = f }
}

func (s *OAuthHandlerTestSuite) validate() error {
	if s.adapter == nil {
		return errors.New("adapter is nil")
	}

	return nil
}

func (s *OAuthHandlerTestSuite) afterTest(t *testing.T) {
	t.Helper()

	if s.cleanup != nil {
		if err := s.cleanup(); err != nil {
			t.Fatalf("cleanup failed: %v", err)
		}
	}
}

func (s *OAuthHandlerTestSuite) TestRegisterClient(t *testing.T) {
	type testCase struct {
		setupServer  func(t *testing.T) *httptest.Server
		originURL    func(srv *httptest.Server) *url.URL
		redirectURL  *url.URL
		opts         func(srv *httptest.Server) []RegisterClientOption
		assertErr    func(t *testing.T, err error)
		assertResult func(t *testing.T, srv *httptest.Server, cfg *oauth2.Config, expiresAt time.Time)
		name         string
	}

	tests := []testCase{
		{
			name: "success with complete dynamic flow",
			setupServer: func(t *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")

					switch r.URL.Path {
					case "/.well-known/oauth-protected-resource":
						// Return authorization server metadata
						_ = json.NewEncoder(w).Encode(map[string]any{
							"authorization_servers": []string{"http://" + r.Host},
						})
					case "/.well-known/oauth-authorization-server":
						// Return registration endpoint
						_ = json.NewEncoder(w).Encode(map[string]any{
							"authorization_endpoint": "http://" + r.Host + "/auth",
							"token_endpoint":         "http://" + r.Host + "/token",
							"registration_endpoint":  "http://" + r.Host + "/register",
						})
					case "/register":
						assert.Equal(t, http.MethodPost, r.Method)
						w.WriteHeader(http.StatusCreated)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"client_id":     "client-123",
							"client_secret": "secret-123",
						})
					default:
						w.WriteHeader(http.StatusNotFound)
					}
				}))
			},
			originURL: func(srv *httptest.Server) *url.URL {
				u, _ := url.Parse(srv.URL)
				return u
			},
			redirectURL: must(url.Parse("http://localhost/callback")),
			assertResult: func(t *testing.T, srv *httptest.Server, cfg *oauth2.Config, expiresAt time.Time) {
				require.NotNil(t, cfg)
				assert.Equal(t, "client-123", cfg.ClientID)
				assert.Equal(t, "secret-123", cfg.ClientSecret)
				assert.Equal(t, "http://localhost/callback", cfg.RedirectURL)
				assert.Equal(t, "http://"+srv.Listener.Addr().String()+"/auth", cfg.Endpoint.AuthURL)
				assert.Equal(t, "http://"+srv.Listener.Addr().String()+"/token", cfg.Endpoint.TokenURL)
			},
			opts:      nil,
			assertErr: nil,
		},
		{
			name: "fallback to domain if protected resource is missing",
			setupServer: func(t *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")

					switch r.URL.Path {
					case "/.well-known/oauth-authorization-server":
						_ = json.NewEncoder(w).Encode(map[string]any{
							"registration_endpoint": "http://" + r.Host + "/register",
						})
					case "/register":
						w.WriteHeader(http.StatusCreated)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"client_id": "fallback-client",
						}) // no client_secret
					default:
						w.WriteHeader(http.StatusNotFound)
					}
				}))
			},
			originURL: func(srv *httptest.Server) *url.URL {
				u, _ := url.Parse(srv.URL)
				return u
			},
			redirectURL: must(url.Parse("http://localhost/callback")),
			assertResult: func(t *testing.T, srv *httptest.Server, cfg *oauth2.Config, expiresAt time.Time) {
				require.NotNil(t, cfg)
				assert.Equal(t, "fallback-client", cfg.ClientID)
				assert.Empty(t, cfg.ClientSecret)
			},
			opts:      nil,
			assertErr: nil,
		},
		{
			name: "success with suggested protected resource option",
			setupServer: func(t *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")

					switch r.URL.Path {
					case "/custom-protected-resource":
						_ = json.NewEncoder(w).Encode(map[string]any{
							"authorization_servers": []string{"http://" + r.Host},
						})
					case "/.well-known/oauth-authorization-server":
						_ = json.NewEncoder(w).Encode(map[string]any{
							"registration_endpoint": "http://" + r.Host + "/register",
						})
					case "/register":
						w.WriteHeader(http.StatusCreated)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"client_id": "suggested-client",
						})
					default:
						w.WriteHeader(http.StatusNotFound)
					}
				}))
			},
			originURL: func(srv *httptest.Server) *url.URL {
				u, _ := url.Parse("http://some-fake-domain.com") // Original domain should be ignored
				return u
			},
			redirectURL: must(url.Parse("http://localhost/callback")),
			opts: func(srv *httptest.Server) []RegisterClientOption {
				u, _ := url.Parse(srv.URL + "/custom-protected-resource")
				return []RegisterClientOption{WithSuggestedProtectedResource(u)}
			},
			assertResult: func(t *testing.T, srv *httptest.Server, cfg *oauth2.Config, expiresAt time.Time) {
				require.NotNil(t, cfg)
				assert.Equal(t, "suggested-client", cfg.ClientID)
			},
			assertErr: nil,
		},
		{
			name: "error dynamic client registration not supported",
			setupServer: func(t *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")

					switch r.URL.Path {
					case "/.well-known/oauth-protected-resource":
						_ = json.NewEncoder(w).Encode(map[string]any{
							"resource_documentation": "https://developer.example.com",
							"authorization_servers":  []string{"http://" + r.Host},
						})
					case "/.well-known/oauth-authorization-server":
						_ = json.NewEncoder(w).Encode(map[string]any{
							"authorization_endpoint": "http://" + r.Host + "/auth",
							"token_endpoint":         "http://" + r.Host + "/token",
						}) // no registration_endpoint
					default:
						w.WriteHeader(http.StatusNotFound)
					}
				}))
			},
			originURL: func(srv *httptest.Server) *url.URL {
				u, _ := url.Parse(srv.URL)
				return u
			},
			redirectURL: must(url.Parse("http://localhost/callback")),
			assertErr: func(t *testing.T, err error) {
				require.Error(t, err)

				var expectedErr *DynamicClientRegistrationNotSupportedError
				require.ErrorAs(t, err, &expectedErr)
				require.NotNil(t, expectedErr.Documentation())
				assert.Equal(t, "https://developer.example.com", expectedErr.Documentation().String())
			},
			opts:         nil,
			assertResult: nil,
		},
		{
			name: "error formatting invalid server URL",
			setupServer: func(t *testing.T) *httptest.Server {
				return nil // no server needed
			},
			originURL: func(srv *httptest.Server) *url.URL {
				u, _ := url.Parse("http://%%invalid")
				return u
			},
			redirectURL: must(url.Parse("http://localhost/callback")),
			assertErr: func(t *testing.T, err error) {
				require.Error(t, err)
			},
			opts:         nil,
			assertResult: nil,
		},
		{
			name: "error origin url is nil",
			originURL: func(srv *httptest.Server) *url.URL {
				return nil
			},
			redirectURL: must(url.Parse("http://localhost/callback")),
			assertErr: func(t *testing.T, err error) {
				require.ErrorContains(t, err, "origin url is nil")
			},
			opts:         nil,
			setupServer:  nil,
			assertResult: nil,
		},
		{
			name: "error redirect url is nil",
			originURL: func(srv *httptest.Server) *url.URL {
				u, _ := url.Parse("http://localhost")
				return u
			},
			redirectURL: nil,
			assertErr: func(t *testing.T, err error) {
				require.ErrorContains(t, err, "redirect url is nil")
			},
			setupServer:  nil,
			opts:         nil,
			assertResult: nil,
		},
		{
			name: "error broken server response on metadata",
			setupServer: func(t *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{ "authorization_servers": [`)) // malformed json
				}))
			},
			originURL: func(srv *httptest.Server) *url.URL {
				u, _ := url.Parse(srv.URL)
				return u
			},
			redirectURL: must(url.Parse("http://localhost/callback")),
			assertErr: func(t *testing.T, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, "failed to get authorization server metadata")
			},
			opts:         nil,
			assertResult: nil,
		},
		{
			name: "error returned during registration (403)",
			setupServer: func(t *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")

					switch r.URL.Path {
					case "/.well-known/oauth-authorization-server":
						_ = json.NewEncoder(w).Encode(map[string]any{
							"registration_endpoint": "http://" + r.Host + "/register",
						})
					case "/register":
						w.WriteHeader(http.StatusForbidden)
						_ = json.NewEncoder(w).Encode(map[string]any{
							"error": "insufficient_permissions",
						})
					default:
						w.WriteHeader(http.StatusNotFound)
					}
				}))
			},
			originURL: func(srv *httptest.Server) *url.URL {
				u, _ := url.Parse(srv.URL)
				return u
			},
			redirectURL: must(url.Parse("http://localhost/callback")),
			assertErr: func(t *testing.T, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, "unexpected status code 403")
				require.ErrorContains(t, err, "insufficient_permissions")
			},
			opts:         nil,
			assertResult: nil,
		},
		{
			name: "error broken urls inside metadata",
			setupServer: func(t *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")

					switch r.URL.Path {
					case "/.well-known/oauth-authorization-server":
						_ = json.NewEncoder(w).Encode(map[string]any{
							"registration_endpoint": "http://%%invalid_registration",
						})
					default:
						w.WriteHeader(http.StatusNotFound)
					}
				}))
			},
			originURL: func(srv *httptest.Server) *url.URL {
				u, _ := url.Parse(srv.URL)
				return u
			},
			redirectURL: must(url.Parse("http://localhost/callback")),
			assertErr: func(t *testing.T, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, "failed to parse registration endpoint")
			},
			opts:         nil,
			assertResult: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var srv *httptest.Server
			if tc.setupServer != nil {
				srv = tc.setupServer(t)
			}

			if srv != nil {
				defer srv.Close()
			}

			origin := new(url.URL)
			if tc.originURL != nil {
				origin = tc.originURL(srv)
			}

			redirect := tc.redirectURL

			var opts []RegisterClientOption
			if tc.opts != nil {
				opts = tc.opts(srv)
			}

			cfg, expiresAt, err := s.adapter.RegisterClient(t.Context(), origin, "test-client", redirect, opts...)

			if tc.assertErr != nil {
				tc.assertErr(t, err)
			} else {
				require.NoError(t, err)
			}

			if tc.assertResult != nil {
				tc.assertResult(t, srv, cfg, expiresAt)
			}
		})
	}
}

// TestRefreshToken tests refreshing OAuth tokens.
func (s *OAuthHandlerTestSuite) TestRefreshToken(t *testing.T) {
	// TODO: implement tests for RefreshToken
}

// TestExchange tests exchanging authorization code with PKCE support.
func (s *OAuthHandlerTestSuite) TestExchange(t *testing.T) {
	// TODO: implement tests for Exchange
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}

	return v
}

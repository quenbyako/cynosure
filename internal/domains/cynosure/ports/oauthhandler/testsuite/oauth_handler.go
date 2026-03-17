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

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports/oauthhandler"
)

const (
	wellKnownOAuthProtectedResourcePath   = "/.well-known/oauth-protected-resource"
	wellKnownOAuthAuthorizationServerPath = "/.well-known/oauth-authorization-server"
	registerPath                          = "/register"
)

// RunOAuthHandlerTests runs tests for the given adapter. These tests are predefined
// and REQUIRED to be used for ANY adapter implementation.
//
// After constructing test function, just run it through `run(t)` call,
// everything else will be handled for you. Calling this function through
// `t.Run("general", run)` is not very recommended, cause test logs will be too
// hard to read cause of big nesting.
func RunOAuthHandlerTests(
	a oauthhandler.Port, opts ...OAuthHandlerTestSuiteOption,
) func(t *testing.T) {
	suite := &OAuthHandlerTestSuite{
		adapter: a,
		cleanup: nil,
	}
	for _, opt := range opts {
		opt(suite)
	}

	if err := suite.validate(); err != nil {
		panic(err) //nolint:forbidigo // ok for tests
	}

	return runSuite(suite)
}

type OAuthHandlerTestSuite struct {
	adapter oauthhandler.Port

	cleanup func() error
}

var _ afterTest = (*OAuthHandlerTestSuite)(nil)

type OAuthHandlerTestSuiteOption func(*OAuthHandlerTestSuite)

func WithOAuthHandlerCleanup(f func() error) OAuthHandlerTestSuiteOption {
	return func(s *OAuthHandlerTestSuite) { s.cleanup = f }
}

func (s *OAuthHandlerTestSuite) validate() error {
	if s.adapter == nil {
		return errors.New("adapter is nil") //nolint:err113 // ok for tests
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

type (
	registerClientOpts = []oauthhandler.RegisterClientOption
	setupServerFunc    = func(t *testing.T) (*httptest.Server, *url.URL, registerClientOpts)
)

//nolint:gocognit,cyclop,funlen,maintidx // we will completely rewrite this test later.
func (s *OAuthHandlerTestSuite) TestRegisterClient(t *testing.T) {
	type testCase struct {
		setupServer  setupServerFunc
		redirectURL  *url.URL
		assertErr    func(t *testing.T, err error)
		assertResult func(
			t *testing.T, srv *httptest.Server, cfg *oauth2.Config, expiresAt time.Time,
		)
		name string
	}

	tests := []testCase{
		{
			name: "success with complete dynamic flow",
			setupServer: func(t *testing.T) (*httptest.Server, *url.URL, registerClientOpts) {
				t.Helper()

				srv := httptest.NewServer(http.HandlerFunc(func(
					w http.ResponseWriter, r *http.Request,
				) {
					w.Header().Set("Content-Type", "application/json")

					switch r.URL.Path {
					case wellKnownOAuthProtectedResourcePath:
						//nolint:errcheck,gosec // makes no sense to check error here
						json.NewEncoder(w).Encode(map[string]any{
							"authorization_servers": []string{"http://" + r.Host},
						})
					case wellKnownOAuthAuthorizationServerPath:
						//nolint:errcheck,gosec // makes no sense to check error here
						json.NewEncoder(w).Encode(map[string]any{
							"authorization_endpoint": "http://" + r.Host + "/auth",
							"token_endpoint":         "http://" + r.Host + "/token",
							"registration_endpoint":  "http://" + r.Host + "/register",
						})
					case registerPath:
						// using assert cause http handler
						if !assert.Equal(t, http.MethodPost, r.Method) {
							return
						}

						w.WriteHeader(http.StatusCreated)

						//nolint:errcheck,gosec // makes no sense to check error here
						json.NewEncoder(w).Encode(map[string]any{
							"client_id":     "client-123",
							"client_secret": "secret-123",
						})
					default:
						w.WriteHeader(http.StatusNotFound)
					}
				}))
				addr, err := url.Parse(srv.URL)
				require.NoError(t, err)

				return srv, addr, nil
			},
			redirectURL: must(url.Parse("http://localhost/callback")),
			assertResult: func(
				t *testing.T, srv *httptest.Server, cfg *oauth2.Config, expiresAt time.Time,
			) {
				t.Helper()

				require.NotNil(t, cfg)
				require.Equal(t, "client-123", cfg.ClientID)
				require.Equal(t, "secret-123", cfg.ClientSecret)
				require.Equal(t, "http://localhost/callback", cfg.RedirectURL)
				require.Equal(
					t, "http://"+srv.Listener.Addr().String()+"/auth", cfg.Endpoint.AuthURL,
				)
				require.Equal(
					t, "http://"+srv.Listener.Addr().String()+"/token", cfg.Endpoint.TokenURL,
				)
			},
			assertErr: nil,
		},
		{
			name: "fallback to domain if protected resource is missing",
			setupServer: func(t *testing.T) (*httptest.Server, *url.URL, registerClientOpts) {
				t.Helper()

				srv := httptest.NewServer(http.HandlerFunc(func(
					w http.ResponseWriter, r *http.Request,
				) {
					w.Header().Set("Content-Type", "application/json")

					switch r.URL.Path {
					case wellKnownOAuthAuthorizationServerPath:
						//nolint:errcheck,gosec // makes no sense to check error here
						json.NewEncoder(w).Encode(map[string]any{
							"registration_endpoint": "http://" + r.Host + "/register",
						})
					case registerPath:
						w.WriteHeader(http.StatusCreated)
						//nolint:errcheck,gosec // makes no sense to check error here
						json.NewEncoder(w).Encode(map[string]any{
							"client_id": "fallback-client",
						}) // no client_secret
					default:
						w.WriteHeader(http.StatusNotFound)
					}
				}))
				addr, err := url.Parse(srv.URL)
				require.NoError(t, err)

				return srv, addr, nil
			},
			redirectURL: must(url.Parse("http://localhost/callback")),
			assertResult: func(
				t *testing.T, srv *httptest.Server, cfg *oauth2.Config, expiresAt time.Time,
			) {
				t.Helper()

				require.NotNil(t, cfg)
				require.Equal(t, "fallback-client", cfg.ClientID)
				require.Empty(t, cfg.ClientSecret)
			},
			assertErr: nil,
		},
		{
			name: "success with suggested protected resource option",
			setupServer: func(t *testing.T) (*httptest.Server, *url.URL, registerClientOpts) {
				t.Helper()

				srv := httptest.NewServer(http.HandlerFunc(func(
					w http.ResponseWriter, r *http.Request,
				) {
					w.Header().Set("Content-Type", "application/json")

					switch r.URL.Path {
					case "/custom-protected-resource":
						//nolint:errcheck,gosec // makes no sense to check error here
						json.NewEncoder(w).Encode(map[string]any{
							"authorization_servers": []string{"http://" + r.Host},
						})
					case wellKnownOAuthAuthorizationServerPath:
						//nolint:errcheck,gosec // makes no sense to check error here
						json.NewEncoder(w).Encode(map[string]any{
							"registration_endpoint": "http://" + r.Host + "/register",
						})
					case "/register":
						w.WriteHeader(http.StatusCreated)
						//nolint:errcheck,gosec // makes no sense to check error here
						json.NewEncoder(w).Encode(map[string]any{
							"client_id": "suggested-client",
						})
					default:
						w.WriteHeader(http.StatusNotFound)
					}
				}))

				// Original domain should be ignored
				addr, err := url.Parse("http://some-fake-domain.com")
				require.NoError(t, err)

				protectedAddr, err := url.Parse(srv.URL + "/custom-protected-resource")
				require.NoError(t, err)

				opts := []oauthhandler.RegisterClientOption{
					oauthhandler.WithSuggestedProtectedResource(protectedAddr),
				}

				return srv, addr, opts
			},
			redirectURL: must(url.Parse("http://localhost/callback")),

			assertResult: func(
				t *testing.T, srv *httptest.Server, cfg *oauth2.Config, expiresAt time.Time,
			) {
				t.Helper()

				require.NotNil(t, cfg)
				require.Equal(t, "suggested-client", cfg.ClientID)
			},
			assertErr: nil,
		},
		{
			name: "error dynamic client registration not supported",
			setupServer: func(t *testing.T) (*httptest.Server, *url.URL, registerClientOpts) {
				t.Helper()

				srv := httptest.NewServer(http.HandlerFunc(func(
					w http.ResponseWriter, r *http.Request,
				) {
					w.Header().Set("Content-Type", "application/json")

					switch r.URL.Path {
					case wellKnownOAuthProtectedResourcePath:
						//nolint:errcheck,gosec // makes no sense to check error here
						json.NewEncoder(w).Encode(map[string]any{
							"resource_documentation": "https://developer.example.com",
							"authorization_servers":  []string{"http://" + r.Host},
						})
					case wellKnownOAuthAuthorizationServerPath:
						//nolint:errcheck,gosec // makes no sense to check error here
						json.NewEncoder(w).Encode(map[string]any{
							"authorization_endpoint": "http://" + r.Host + "/auth",
							"token_endpoint":         "http://" + r.Host + "/token",
						}) // no registration_endpoint
					default:
						w.WriteHeader(http.StatusNotFound)
					}
				}))
				addr, err := url.Parse(srv.URL)
				require.NoError(t, err)

				return srv, addr, nil
			},
			redirectURL: must(url.Parse("http://localhost/callback")),
			assertErr: func(t *testing.T, err error) {
				t.Helper()

				require.Error(t, err)

				var expectedErr *oauthhandler.DynamicClientRegistrationNotSupportedError
				require.ErrorAs(t, err, &expectedErr)
				require.NotNil(t, expectedErr.Documentation())
				require.Equal(t, "https://developer.example.com",
					expectedErr.Documentation().String())
			},
			assertResult: nil,
		},
		{
			name: "error formatting invalid server URL",
			setupServer: func(t *testing.T) (*httptest.Server, *url.URL, registerClientOpts) {
				t.Helper()

				//nolint:exhaustruct // just simulating invalid url, no need to fill all fields
				return nil, &url.URL{
					Scheme: "http",
					Host:   "%%invalid",
				}, nil
			},
			redirectURL: must(url.Parse("http://localhost/callback")),
			assertErr: func(t *testing.T, err error) {
				t.Helper()

				require.Error(t, err)
			},
			assertResult: nil,
		},
		{
			name: "error origin url is nil",
			setupServer: func(t *testing.T) (*httptest.Server, *url.URL, registerClientOpts) {
				t.Helper()

				return nil, nil, nil
			},
			redirectURL: must(url.Parse("http://localhost/callback")),
			assertErr: func(t *testing.T, err error) {
				t.Helper()

				require.ErrorContains(t, err, "origin url is nil")
			},
			assertResult: nil,
		},
		{
			name: "error redirect url is nil",
			setupServer: func(t *testing.T) (*httptest.Server, *url.URL, registerClientOpts) {
				t.Helper()

				addr, err := url.Parse("http://localhost")
				require.NoError(t, err)

				return nil, addr, nil
			},
			redirectURL: nil,
			assertErr: func(t *testing.T, err error) {
				t.Helper()

				require.ErrorContains(t, err, "redirect url is nil")
			},
			assertResult: nil,
		},
		{
			name: "error broken server response on metadata",
			setupServer: func(t *testing.T) (*httptest.Server, *url.URL, registerClientOpts) {
				t.Helper()

				srv := httptest.NewServer(http.HandlerFunc(func(
					w http.ResponseWriter, r *http.Request,
				) {
					w.Header().Set("Content-Type", "application/json")
					//nolint:errcheck,gosec // makes no sense to check error here
					w.Write([]byte(`{ "authorization_servers": [`)) // malformed json
				}))

				addr, err := url.Parse(srv.URL)
				require.NoError(t, err)

				return srv, addr, nil
			},
			redirectURL: must(url.Parse("http://localhost/callback")),
			assertErr: func(t *testing.T, err error) {
				t.Helper()

				require.Error(t, err)
				require.ErrorContains(t, err, "failed to get authorization server metadata")
			},
			assertResult: nil,
		},
		{
			name: "error returned during registration (403)",
			setupServer: func(t *testing.T) (*httptest.Server, *url.URL, registerClientOpts) {
				t.Helper()

				srv := httptest.NewServer(http.HandlerFunc(func(
					w http.ResponseWriter, r *http.Request,
				) {
					w.Header().Set("Content-Type", "application/json")

					switch r.URL.Path {
					case "/.well-known/oauth-authorization-server":
						//nolint:errcheck,gosec // makes no sense to check error here
						json.NewEncoder(w).Encode(map[string]any{
							"registration_endpoint": "http://" + r.Host + "/register",
						})
					case "/register":
						w.WriteHeader(http.StatusForbidden)
						//nolint:errcheck,gosec // makes no sense to check error here
						json.NewEncoder(w).Encode(map[string]any{
							"error": "insufficient_permissions",
						})
					default:
						w.WriteHeader(http.StatusNotFound)
					}
				}))
				addr, err := url.Parse(srv.URL)
				require.NoError(t, err)

				return srv, addr, nil
			},
			redirectURL: must(url.Parse("http://localhost/callback")),
			assertErr: func(t *testing.T, err error) {
				t.Helper()

				require.Error(t, err)
				require.ErrorContains(t, err, "unexpected status code 403")
				require.ErrorContains(t, err, "insufficient_permissions")
			},
			assertResult: nil,
		},
		{
			name: "error broken urls inside metadata",
			setupServer: func(t *testing.T) (*httptest.Server, *url.URL, registerClientOpts) {
				t.Helper()

				srv := httptest.NewServer(http.HandlerFunc(func(
					w http.ResponseWriter, r *http.Request,
				) {
					w.Header().Set("Content-Type", "application/json")

					switch r.URL.Path {
					case "/.well-known/oauth-authorization-server":
						//nolint:errcheck,gosec // makes no sense to check error here
						json.NewEncoder(w).Encode(map[string]any{
							"registration_endpoint": "http://%%invalid_registration",
						})
					default:
						w.WriteHeader(http.StatusNotFound)
					}
				}))

				addr, err := url.Parse(srv.URL)
				require.NoError(t, err)

				return srv, addr, nil
			},
			redirectURL: must(url.Parse("http://localhost/callback")),
			assertErr: func(t *testing.T, err error) {
				t.Helper()

				require.Error(t, err)
				require.ErrorContains(t, err, "failed to parse registration endpoint")
			},
			assertResult: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var (
				srv    *httptest.Server
				origin *url.URL
				opts   []oauthhandler.RegisterClientOption
			)
			if tc.setupServer != nil {
				srv, origin, opts = tc.setupServer(t)
			}

			if srv != nil {
				defer srv.Close()
			}

			redirect := tc.redirectURL

			cfg, expiresAt, err := s.adapter.RegisterClient(
				t.Context(), origin, "test-client", redirect, opts...,
			)

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
		panic(err) //nolint:forbidigo
	}

	return v
}

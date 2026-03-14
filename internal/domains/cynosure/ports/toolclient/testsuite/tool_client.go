package testsuite

import (
	"iter"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/quenbyako/cynosure/contrib/mcpmock"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"

	. "github.com/quenbyako/cynosure/internal/domains/cynosure/ports/toolclient"
)

// RunToolClientTests runs tests for the given adapter. These tests are
// predefined and recommended to be used for ANY adapter implementation.
func RunToolClientTests(a Port, opts ...ToolClientTestSuiteOpts) func(t *testing.T) {
	s := &ToolClientTestSuite{
		adapter: a,
	}
	for _, opt := range opts {
		opt(s)
	}

	return runSuite(s)
}

type ToolClientTestSuite struct {
	adapter Port

	afterServerSetup func(u *url.URL)
}

type ToolClientTestSuiteOpts func(*ToolClientTestSuite)

func WithAfterServerSetup(f func(u *url.URL)) ToolClientTestSuiteOpts {
	return func(s *ToolClientTestSuite) { s.afterServerSetup = f }
}

func (s *ToolClientTestSuite) TestProbe(t *testing.T) {
	srvAccountMaker := map[string]func(srv *mcpmock.MockServer) *oauth2.Token{
		"no_account": func(srv *mcpmock.MockServer) *oauth2.Token {
			return nil
		},
		"valid_account": func(srv *mcpmock.MockServer) *oauth2.Token {
			token := "someValidToken=="
			srv.IssueToken(token)

			return &oauth2.Token{
				AccessToken: token,
			}
		},
		"invalid_account": func(srv *mcpmock.MockServer) *oauth2.Token {
			token := "someValidToken=="
			srv.IssueToken(token)

			return &oauth2.Token{
				AccessToken: "wrongToken==",
			}
		},
		//TODO: test this too.
		// "invalid_token_valid_refresh": func(srv *mcpmock.MockServer) *oauth2.Token {
		//	token := "someValidToken=="
		//	srv.IssueToken(token)
		//
		//	return &oauth2.Token{
		//		AccessToken: "wrongToken==",
		//	}
		// },

	}

	name := "some-account"
	desc := "Some Account Description"

	for config := range configIterator() {
		for testName, tokenMaker := range srvAccountMaker {
			t.Run(config.String()+"/"+testName, func(t *testing.T) {
				t.Parallel()

				account := dummyAccount()

				srv := mcpmock.New(config)
				defer srv.Close()

				token := tokenMaker(srv)

				if s.afterServerSetup != nil {
					s.afterServerSetup(srv.MCPURL())
				}

				nameToID := map[string]ids.ToolID{}

				tools, err := s.adapter.DiscoverTools(t.Context(), srv.MCPURL(), account, name, desc, WithToolIDBuilder(func(account ids.AccountID, name string) (ids.ToolID, error) {
					if _, ok := nameToID[name]; ok {
						panic("unexpected tool name collision")
					}

					id := must(ids.RandomToolID(account))
					nameToID[name] = id

					return id, nil
				}), WithAuthToken(token))
				expectError := config.Auth == mcpmock.AuthRequired || config.Auth == mcpmock.AuthNoHeader
				// If we have a valid account and the server uses Bearer (which oauth defaults to), it will succeed.
				if testName == "valid_account" && (config.AuthType == mcpmock.AuthTypeNone || config.AuthType == mcpmock.AuthTypeBearer) {
					expectError = false
				}

				if expectError {
					if testName == "invalid_account" {
						// when we have invalid account, we are throwing generic
						// errors. adapter MUST implement automatic refresh at
						// least once.
						require.ErrorIs(t, err, ErrInvalidCredentials)
						return
					}

					e := new(RequiresAuthError)
					require.ErrorAs(t, err, &e)

					// Endpoint is only provided by server if it sends WWW-Authenticate header.
					// For AuthNoHeader, server does not send it, so Endpoint() will naturally be nil.
					if config.Protected == mcpmock.MetadataDiscoveryPathExplicit && config.Auth != mcpmock.AuthNoHeader {
						require.NotNil(t, e.Endpoint())
						require.NotEmpty(t, e.Endpoint().String())
					}

					return
				}

				if require.NoError(t, err); err != nil {
					return
				}

				require.ElementsMatch(t, srv.Tools(name, desc, func(name string) ids.ToolID { return nameToID[name] }), tools)
			})
		}
	}
}

func dummyAccount() ids.AccountID {
	user := ids.RandomUserID()
	server, _ := ids.NewServerID(uuid.New())
	account, _ := ids.NewAccountID(user, server, uuid.New())

	return account
}

func configIterator() iter.Seq[mcpmock.MockServerConfig] {
	transports := [...]mcpmock.TransportType{mcpmock.TransportSSE, mcpmock.TransportHTTP}
	authReqs := [...]mcpmock.AuthRequirement{mcpmock.AuthNone, mcpmock.AuthOptional, mcpmock.AuthRequired, mcpmock.AuthNoHeader}
	metadataDiscovery := [...]mcpmock.MetadataDiscovery{mcpmock.MetadataDiscoveryNone, mcpmock.MetadataDiscoveryRoot, mcpmock.MetadataDiscoveryPathSpecific, mcpmock.MetadataDiscoveryPathExplicit}
	oauthDiscobery := [...]mcpmock.OAuthDiscovery{mcpmock.OAuthDiscoveryNone}                     // , mcpmock.OAuthDiscoveryAuthOnly, mcpmock.OAuthDiscoveryFull}
	metadataRegistration := [...]mcpmock.OAuthRegistration{mcpmock.OAuthRegistrationNotSupported} // , mcpmock.OAuthRegistrationSupported}
	oauthFlow := [...]mcpmock.OAuthFlow{mcpmock.OAuthFlowReturnsURL}                              // , mcpmock.OAuthFlowAuthLinkError, mcpmock.OAuthFlowInstantlyReturnsToken}
	metadataToken := [...]mcpmock.OAuthTokenStatus{mcpmock.TokenValid}                            // , mcpmock.TokenExpiredRefreshValid, mcpmock.TokenExpiredRefreshExpired}
	metadataAuthType := [...]mcpmock.AuthType{mcpmock.AuthTypeNone}                               // , mcpmock.AuthTypeBearer, mcpmock.AuthTypeRandomized}

	return func(yield func(mcpmock.MockServerConfig) bool) {
	globalIter:
		for _, transport := range transports {
			for _, auth := range authReqs {
				for _, discovery := range metadataDiscovery {
					for _, oauthDiscovery := range oauthDiscobery {
						for _, registration := range metadataRegistration {
							for _, flow := range oauthFlow {
								for _, token := range metadataToken {
									for _, authType := range metadataAuthType {
										ok := yield(mcpmock.MockServerConfig{
											Transport:    transport,
											Auth:         auth,
											Discovery:    oauthDiscovery,
											Registration: registration,
											Flow:         flow,
											Token:        token,
											Protected:    discovery,
											AuthType:     authType,
										})
										if !ok {
											break globalIter
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}
}

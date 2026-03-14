package mcpmock

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
	"github.com/quenbyako/ext/syncmap"
)

func randomString(prefix string, n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(b))
}

func randomPath(prefix string) string {
	return "/" + randomString(prefix, 4)
}

// MockServerConfig defines the behavior and features of the Mock MCP Server.
type MockServerConfig struct {
	Transport    TransportType
	Auth         AuthRequirement
	Discovery    OAuthDiscovery
	Registration OAuthRegistration
	Flow         OAuthFlow
	Token        OAuthTokenStatus
	Protected    MetadataDiscovery
	AuthType     AuthType
}

func (c MockServerConfig) String() string {
	return fmt.Sprintf("transport=%q,auth=%q,discovery=%q,registration=%q,flow=%q,token=%q,protected=%q,authType=%q",
		c.Transport, c.Auth, c.Discovery, c.Registration, c.Flow, c.Token, c.Protected, c.AuthType)
}

// MockServer wraps httptest.Server and manages its states.
type MockServer struct {
	*httptest.Server
	cfg MockServerConfig

	// authTypeString is the actual string used in headers (e.g. "Bearer" or "Random-X")
	authTypeString string

	// tokens maps token string to its status (valid/invalid)
	tokens syncmap.Map[string, struct{}]
	// authCodes maps code to token (simulating auth flow)
	authCodes syncmap.Map[string, string]

	mcpPath      string
	metadataPath string
}

func (m *MockServer) MCPURL() *url.URL {
	return must(url.Parse(m.Server.URL + m.mcpPath))
}

func (m *MockServer) IssueToken(token string) {
	m.tokens.Store(token, struct{}{})
}

func (m *MockServer) RevokeToken(token string) {
	m.tokens.Delete(token)
}

func (m *MockServer) DropConnections() {
	m.Server.CloseClientConnections()
}

func (m *MockServer) Tools(accountName, accountDesc string, toolID func(string) ids.ToolID) []tools.RawTool {
	return []tools.RawTool{
		must(tools.NewRawTool(
			"mock_tool",
			"A mock tool",
			json.RawMessage(`{"type":"object"}`),
			json.RawMessage(`{"type":"string"}`),
			toolID("mock_tool"), accountName, accountDesc,
		)),
	}
}
func mockMCPServer() *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "mock-mcp-server",
		Version: "1.0.0",
	}, nil)

	srv.AddTool(&mcp.Tool{
		Name:        "mock_tool",
		Description: "A mock tool",
		InputSchema: map[string]any{"type": "object"},
	}, func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "mock response"},
			},
		}, nil
	})

	return srv
}

func New(cfg MockServerConfig) *MockServer {
	mux := http.NewServeMux()

	mcpPath := randomPath("mcp")
	authPath := randomPath("auth")
	tokenPath := randomPath("token")
	regPath := randomPath("reg")

	var authTypeStr string
	switch cfg.AuthType {
	case AuthTypeNone:
		authTypeStr = ""
	case AuthTypeBearer:
		authTypeStr = "Bearer"
	case AuthTypeRandomized:
		authTypeStr = randomString("Auth", 2)
	default:
		panic(fmt.Sprintf("unknown auth type: %v", cfg.AuthType))
	}

	srv := &MockServer{
		cfg:            cfg,
		authTypeString: authTypeStr,
		mcpPath:        mcpPath,
	}

	// For middleware, we need a prefix. If AuthTypeNone, we assume Bearer for the actual header check.
	effectiveAuthType := authTypeStr
	if effectiveAuthType == "" {
		effectiveAuthType = "Bearer"
	}

	// 2. Setup MCP Server (internal logic)
	mcpServer := mockMCPServer()

	var mcpHandler http.Handler
	switch cfg.Transport {
	case TransportSSE:
		mcpHandler = mcp.NewSSEHandler(func(*http.Request) *mcp.Server { return mcpServer }, nil)
	case TransportHTTP:
		mcpHandler = mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return mcpServer }, nil)
	default:
		panic(fmt.Sprintf("unknown transport type: %v", cfg.Transport))
	}
	mux.Handle(mcpPath, mcpHandler)
	mux.Handle(mcpPath+"/", mcpHandler)

	// 3. Discovery Helpers
	getDiscoveryData := func() map[string]any {
		// Use srv.Server.URL because srv.URL is only available after httptest.NewServer
		baseURL := srv.Server.URL
		res := map[string]any{
			"issuer":                 baseURL,
			"authorization_endpoint": baseURL + authPath,
			"token_endpoint":         baseURL + tokenPath,
		}
		if cfg.Discovery == OAuthDiscoveryFull && cfg.Registration == OAuthRegistrationSupported {
			res["registration_endpoint"] = srv.URL + regPath
		}
		return res
	}

	discoveryHandler := func(w http.ResponseWriter, r *http.Request) {
		if cfg.Discovery == OAuthDiscoveryNone {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(getDiscoveryData())
	}

	// 4. Randomized Metadata (Resource Metadata)
	switch cfg.Protected {
	case MetadataDiscoveryRoot:
		mux.HandleFunc("/.well-known/oauth-authorization-server", discoveryHandler)
	case MetadataDiscoveryPathSpecific:
		mux.HandleFunc("/.well-known/oauth-authorization-server"+srv.mcpPath, discoveryHandler)
	case MetadataDiscoveryPathExplicit:
		srv.metadataPath = randomPath("oauth-metadata")
		mux.HandleFunc(srv.metadataPath, discoveryHandler)
	case MetadataDiscoveryNone:
		// No discovery endpoints
	default:
		panic(fmt.Sprintf("unexpected mcpmock.MetadataDiscovery: %#v", cfg.Protected))
	}

	// 5. Randomized OAuth Routes
	mux.HandleFunc(authPath, func(w http.ResponseWriter, r *http.Request) {
		if cfg.Flow == OAuthFlowAuthLinkError {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		redirectURI := r.URL.Query().Get("redirect_uri")
		state := r.URL.Query().Get("state")
		code := "mock_code_" + state

		srv.authCodes.Store(code, "mock_token_"+state)

		http.Redirect(w, r, redirectURI+"?code="+code+"&state="+state, http.StatusFound)
	})

	mux.HandleFunc(tokenPath, func(w http.ResponseWriter, r *http.Request) {
		code := r.FormValue("code")
		token, ok := srv.authCodes.Load(code)
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		srv.tokens.Store(token, struct{}{})

		resp := map[string]any{
			"access_token": token,
			"expires_in":   3600,
		}
		if srv.authTypeString != "" {
			resp["token_type"] = srv.authTypeString
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	// 6. Final Wiring with Middleware
	checkTokenFunc := func(token string) bool {
		_, ok := srv.tokens.Load(token)
		return ok
	}

	finalHandler := authMiddleware(
		cfg.Auth,
		effectiveAuthType,
		srv.metadataPath,
		func(path string) bool {
			return strings.HasPrefix(path, "/.well-known/") ||
				path == authPath ||
				path == tokenPath ||
				(regPath != "" && path == regPath) ||
				(srv.metadataPath != "" && path == srv.metadataPath)
		},
		checkTokenFunc,
	)(mux)

	srv.Server = httptest.NewServer(finalHandler)
	return srv
}

func authMiddleware(
	auth AuthRequirement,
	authType string,
	customMetadataPath string,
	bypassAuth func(path string) bool,
	checkToken func(token string) bool,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path

			if bypassAuth(path) {
				next.ServeHTTP(w, r)
				return
			}

			if auth == AuthNone || auth == AuthOptional {
				next.ServeHTTP(w, r)
				return
			}

			// Required Auth
			authHeader := r.Header.Get("Authorization")
			prefix := authType + " "
			if strings.HasPrefix(authHeader, prefix) {
				token := strings.TrimPrefix(authHeader, prefix)
				if checkToken(token) {
					next.ServeHTTP(w, r)
					return
				} else {
					w.WriteHeader(http.StatusForbidden)
					return

				}
			}

			if auth == AuthNoHeader {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			// Default Response with WWW-Authenticate
			wwwAuthHeader := fmt.Sprintf(`%s error="invalid_token"`, authType)
			if customMetadataPath != "" {
				wwwAuthHeader += fmt.Sprintf(`, resource_metadata="%s"`, customMetadataPath)
			}

			w.Header().Set("WWW-Authenticate", wwwAuthHeader)
			w.WriteHeader(http.StatusUnauthorized)
		})
	}
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

// Package mcpmock provides a mock MCP server for testing purposes.
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
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/quenbyako/ext/syncmap"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
)

func randomString(prefix string, n int) string {
	b := make([]byte, n)
	rand.Read(b)

	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(b))
}

func randomPath(prefix string) string {
	const randomPathLenHex = 4

	return "/" + randomString(prefix, randomPathLenHex)
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
	return fmt.Sprintf(
		"transport=%q,auth=%q,discovery=%q,registration=%q,"+
			"flow=%q,token=%q,protected=%q,authType=%q",
		c.Transport, c.Auth, c.Discovery, c.Registration, c.Flow, c.Token, c.Protected, c.AuthType,
	)
}

// MockServer wraps httptest.Server and manages its states.
type MockServer struct {
	*httptest.Server
	// tokens maps token string to its status (valid/invalid)
	tokens syncmap.Map[string, struct{}]
	// authCodes maps code to token (simulating auth flow)
	authCodes syncmap.Map[string, string]
	// authTypeString is the actual string used in headers (e.g. "Bearer" or "Random-X")
	authTypeString string
	mcpPath        string
	metadataPath   string
	cfg            MockServerConfig
}

func (m *MockServer) MCPURL() *url.URL {
	return must(url.Parse(m.URL + m.mcpPath))
}

func (m *MockServer) IssueToken(token string) {
	m.tokens.Store(token, struct{}{})
}

func (m *MockServer) RevokeToken(token string) {
	m.tokens.Delete(token)
}

func (m *MockServer) DropConnections() {
	m.CloseClientConnections()
}

func (m *MockServer) Tools(
	accountName, accountDesc string, toolID func(string) ids.ToolID,
) []tools.RawTool {
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
		Name:       "mock-mcp-server",
		Version:    "1.0.0",
		Title:      "",
		WebsiteURL: "",
		Icons:      nil,
	}, nil)

	srv.AddTool(&mcp.Tool{
		Name:         "mock_tool",
		Description:  "A mock tool",
		InputSchema:  map[string]any{"type": "object"},
		Meta:         nil,
		Annotations:  nil,
		OutputSchema: nil,
		Title:        "",
		Icons:        nil,
	}, handleMockTool)

	return srv
}

func handleMockTool(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text:        "mock response",
				Meta:        nil,
				Annotations: nil,
			},
		},
		Meta:              nil,
		StructuredContent: nil,
		IsError:           false,
	}, nil
}

func New(cfg MockServerConfig) *MockServer {
	mux := http.NewServeMux()

	mcpPath := randomPath("mcp")
	authPath := randomPath("auth")
	tokenPath := randomPath("token")
	regPath := randomPath("reg")

	authTypeStr := resolveAuthType(cfg.AuthType)

	srv := newMockServer(cfg, mcpPath, authTypeStr)

	effectiveAuthType := srv.resolveEffectiveAuthType()
	mcpHandler := srv.setupMCPHandler()

	mux.Handle(mcpPath, mcpHandler)
	mux.Handle(mcpPath+"/", mcpHandler)

	srv.setupDiscoveryRoutes(mux, authPath, tokenPath, regPath)
	srv.setupOAuthRoutes(mux, authPath, tokenPath)

	srv.Server = httptest.NewServer(srv.wrapWithMiddleware(
		mux, effectiveAuthType, authPath, tokenPath, regPath,
	))

	return srv
}

func newMockServer(cfg MockServerConfig, mcpPath, authTypeStr string) *MockServer {
	return &MockServer{
		cfg:            cfg,
		authTypeString: authTypeStr,
		mcpPath:        mcpPath,
		Server:         nil,
		tokens:         syncmap.Map[string, struct{}]{},
		authCodes:      syncmap.Map[string, string]{},
		metadataPath:   "",
	}
}

func resolveAuthType(authType AuthType) string {
	switch authType {
	case AuthTypeNone:
		return ""
	case AuthTypeBearer:
		return "Bearer"
	case AuthTypeRandomized:
		const randomBytes = 2

		return randomString("Auth", randomBytes)
	default:
		//nolint:forbidigo // Fatal error in mock server initialization
		panic(fmt.Sprintf("unknown auth type: %v", authType))
	}
}

func (m *MockServer) resolveEffectiveAuthType() string {
	if m.authTypeString == "" {
		return "Bearer"
	}

	return m.authTypeString
}

func (m *MockServer) setupMCPHandler() http.Handler {
	mcpServer := mockMCPServer()

	switch m.cfg.Transport {
	case TransportSSE:
		return mcp.NewSSEHandler(func(*http.Request) *mcp.Server { return mcpServer }, nil)
	case TransportHTTP:
		return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
			return mcpServer
		}, nil)
	default:
		//nolint:forbidigo // Fatal error in mock server initialization
		panic(fmt.Sprintf("unknown transport type: %v", m.cfg.Transport))
	}
}

func (m *MockServer) setupDiscoveryRoutes(mux *http.ServeMux, authPath, tokenPath, regPath string) {
	discoveryHandler := func(w http.ResponseWriter, r *http.Request) {
		if m.cfg.Discovery == OAuthDiscoveryNone {
			w.WriteHeader(http.StatusNotFound)

			return
		}

		w.Header().Set("Content-Type", "application/json")

		//nolint:errcheck // In mock server we don't care about Encode failing
		_ = json.NewEncoder(w).Encode(m.getDiscoveryData(authPath, tokenPath, regPath))
	}

	switch m.cfg.Protected {
	case MetadataDiscoveryRoot:
		mux.HandleFunc("/.well-known/oauth-authorization-server", discoveryHandler)
	case MetadataDiscoveryPathSpecific:
		mux.HandleFunc("/.well-known/oauth-authorization-server"+m.mcpPath, discoveryHandler)
	case MetadataDiscoveryPathExplicit:
		m.metadataPath = randomPath("oauth-metadata")
		mux.HandleFunc(m.metadataPath, discoveryHandler)
	case MetadataDiscoveryNone:
	default:
		//nolint:forbidigo // Fatal error in mock server initialization
		panic(fmt.Sprintf("unexpected mcpmock.MetadataDiscovery: %#v", m.cfg.Protected))
	}
}

func (m *MockServer) getDiscoveryData(authPath, tokenPath, regPath string) map[string]any {
	baseURL := m.URL

	res := map[string]any{
		"issuer":                 baseURL,
		"authorization_endpoint": baseURL + authPath,
		"token_endpoint":         baseURL + tokenPath,
	}

	if m.cfg.Discovery == OAuthDiscoveryFull && m.cfg.Registration == OAuthRegistrationSupported {
		res["registration_endpoint"] = m.URL + regPath
	}

	return res
}

func (m *MockServer) setupOAuthRoutes(mux *http.ServeMux, authPath, tokenPath string) {
	mux.HandleFunc(authPath, func(w http.ResponseWriter, r *http.Request) {
		if m.cfg.Flow == OAuthFlowAuthLinkError {
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		redirectURI := r.URL.Query().Get("redirect_uri")
		state := r.URL.Query().Get("state")
		code := "mock_code_" + state

		m.authCodes.Store(code, "mock_token_"+state)

		http.Redirect(w, r, redirectURI+"?code="+code+"&state="+state, http.StatusFound)
	})

	mux.HandleFunc(tokenPath, m.handleTokenRequest)
}

func (m *MockServer) handleTokenRequest(w http.ResponseWriter, r *http.Request) {
	const maxBodySize = 1 << 20 // 1 MiB

	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	code := r.FormValue("code")

	token, ok := m.authCodes.Load(code)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)

		return
	}

	m.tokens.Store(token, struct{}{})

	const expirationSeconds = int(1 * time.Hour / time.Second)

	resp := map[string]any{
		"access_token": token,
		"expires_in":   expirationSeconds,
	}

	if m.authTypeString != "" {
		resp["token_type"] = m.authTypeString
	}

	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(resp) //nolint:errcheck,gosec // makes no sense
}

func (m *MockServer) wrapWithMiddleware(
	mux *http.ServeMux, authType, authPath, tokenPath, regPath string,
) http.Handler {
	checkTokenFunc := func(token string) bool {
		_, ok := m.tokens.Load(token)

		return ok
	}

	return authMiddleware(
		m.cfg.Auth,
		authType,
		m.metadataPath,
		func(path string) bool {
			return strings.HasPrefix(path, "/.well-known/") ||
				path == authPath ||
				path == tokenPath ||
				(regPath != "" && path == regPath) ||
				(m.metadataPath != "" && path == m.metadataPath)
		},
		checkTokenFunc,
	)(mux)
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
			if bypassAuth(r.URL.Path) || auth == AuthNone || auth == AuthOptional {
				next.ServeHTTP(w, r)

				return
			}

			if checkAuthHeader(w, r, authType, checkToken) {
				next.ServeHTTP(w, r)

				return
			}

			if auth == AuthNoHeader {
				w.WriteHeader(http.StatusUnauthorized)

				return
			}

			writeAuthFailure(w, authType, customMetadataPath)
		})
	}
}

func checkAuthHeader(
	w http.ResponseWriter, r *http.Request, authType string, checkToken func(string) bool,
) bool {
	authHeader := r.Header.Get("Authorization")
	prefix := authType + " "

	if strings.HasPrefix(authHeader, prefix) {
		token := strings.TrimPrefix(authHeader, prefix)
		if checkToken(token) {
			return true
		}

		w.WriteHeader(http.StatusForbidden)

		return false // Handled, but failed
	}

	return false
}

func writeAuthFailure(w http.ResponseWriter, authType, customMetadataPath string) {
	wwwAuthHeader := authType + " error=\"invalid_token\""
	if customMetadataPath != "" {
		wwwAuthHeader += fmt.Sprintf(`, resource_metadata=%q`, customMetadataPath)
	}

	w.Header().Set("WWW-Authenticate", wwwAuthHeader)
	w.WriteHeader(http.StatusUnauthorized)
}

func must[T any](value T, err error) T {
	if err != nil {
		//nolint:forbidigo // Fatal error in mock server initialization
		panic(err)
	}

	return value
}

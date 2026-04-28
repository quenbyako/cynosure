package mcp

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/tools"
)

const (
	noRetries         = -1
	keepAliveInterval = 1 * time.Second
)

type connFactory struct {
	internalTransport      http.RoundTripper
	externalTransport      http.RoundTripper
	tracer                 trace.Tracer
	tokenSourceConstructor TokenSourceConstructor
}

func NewConnectionFactory(
	ctx context.Context,
	tracer trace.Tracer,
	tokenSourcer TokenSourceConstructor,
	externalTransport http.RoundTripper,
	internalTransport http.RoundTripper,
	unsafeExternalTransport bool,
) (*connFactory, error) {
	factory := &connFactory{
		internalTransport:      internalTransport,
		externalTransport:      externalTransport,
		tracer:                 tracer,
		tokenSourceConstructor: tokenSourcer,
	}

	if err := factory.validate(ctx, unsafeExternalTransport); err != nil {
		return nil, err
	}

	return factory, nil
}

func (f *connFactory) validate(ctx context.Context, unsafeExternalTransport bool) error {
	if f.internalTransport == nil || f.externalTransport == nil {
		return fmt.Errorf("factory initialization: %w", ErrTransportsRequired)
	}

	if err := verifySSRF(ctx, f.externalTransport); err != nil && !unsafeExternalTransport {
		return fmt.Errorf("SSRF protection: %w", err)
	}

	return nil
}

func (f *connFactory) buildAnonymousTransport(isInternal bool) http.RoundTripper {
	base := f.externalTransport
	if isInternal {
		base = f.internalTransport
	}

	return authorizeHeaderCollector(base)
}

func (f *connFactory) buildTempAuthTransport(token *oauth2.Token, internal bool) http.RoundTripper {
	base := f.externalTransport
	if internal {
		base = f.internalTransport
	}

	// adding forbidden checker to handle invalid credentials. Since it's
	// impossible to automatically update token, we must throw it as an error.
	return forbiddenChecker(&oauth2.Transport{
		Source: oauth2.StaticTokenSource(token),
		Base:   base,
	})
}

func (f *connFactory) buildAuthorizedTransport(
	id ids.AccountID, config *oauth2.Config, token *oauth2.Token, isInternal bool,
) (http.RoundTripper, error) {
	base := f.externalTransport
	if isInternal {
		base = f.internalTransport
	}

	tokenSource, err := f.tokenSourceConstructor(id, config, token, isInternal)
	if err != nil {
		return nil, fmt.Errorf("building token source: %w", err)
	}

	source := oauth2.ReuseTokenSource(token, tokenSource)

	return &oauth2.Transport{
		Source: source,
		Base:   base,
	}, nil
}

type asyncClient struct {
	cancel       context.CancelFunc
	session      *mcp.ClientSession
	usedProtocol tools.Protocol // Which protocol was successfully used
}

func (f *connFactory) GetAnonymous(
	ctx context.Context, targetURL *url.URL, protocol tools.Protocol, isInternal bool,
) (*asyncClient, error) {
	ctx, span := f.tracer.Start(ctx, "GetAnonymous", trace.WithAttributes(
		attribute.String("mcp.url", targetURL.String()),
	))
	defer span.End()

	clientCtx, clientCancel := context.WithCancel(context.WithoutCancel(ctx))
	client := newHTTPClient(f.buildAnonymousTransport(isInternal))

	session, discovered, err := autoConnectProtocol(
		clientCtx, targetURL.String(), client, protocol,
	)

	return f.finalizeConnect(clientCancel, session, discovered, err)
}

func (f *connFactory) GetPartiallyAuthorized(
	ctx context.Context, targetURL *url.URL, token *oauth2.Token, protocol tools.Protocol,
	isInternal bool,
) (*asyncClient, error) {
	ctx, span := f.tracer.Start(ctx, "GetPartiallyAuthorized", trace.WithAttributes(
		attribute.String("mcp.url", targetURL.String()),
		attribute.Bool("mcp.token_is_nil", token == nil),
	))
	defer span.End()

	if err := validatePartiallyParams(targetURL, token, protocol); err != nil {
		span.RecordError(err)
		return nil, err
	}

	clientCtx, clientCancel := context.WithCancel(context.WithoutCancel(ctx))
	client := newHTTPClient(f.buildTempAuthTransport(token, isInternal))

	session, discovered, err := autoConnectProtocol(
		clientCtx, targetURL.String(), client, protocol,
	)

	return f.finalizeConnect(clientCancel, session, discovered, err)
}

func validatePartiallyParams(u *url.URL, token *oauth2.Token, proto tools.Protocol) error {
	if u == nil {
		return ErrURLIsNil
	}

	if token == nil {
		return ErrTokenIsNil
	}

	if !proto.Valid() {
		return ErrProtocolIsInvalid
	}

	return nil
}

func (f *connFactory) GetAuthorized(
	ctx context.Context, id ids.AccountID,
	server entities.ServerConfigReadOnly, token *oauth2.Token,
) (*asyncClient, error) {
	ctx, span := f.tracer.Start(ctx, "GetAuthorized", trace.WithAttributes(
		attribute.String("mcp.account_id", id.ID().String()),
		attribute.Bool("mcp.token_is_nil", token == nil),
	))
	defer span.End()

	if err := validateAuthorizedParams(id, server, token); err != nil {
		span.RecordError(err)
		return nil, err
	}

	transport, err := f.buildAuthorizedTransport(id, server.AuthConfig(), token, server.Internal())
	if err != nil {
		return nil, err
	}

	clientCtx, clientCancel := context.WithCancel(context.WithoutCancel(ctx))
	session, discovered, err := autoConnectProtocol(
		clientCtx, server.SSELink().String(), newHTTPClient(transport), server.PreferredProtocol(),
	)

	return f.finalizeConnect(clientCancel, session, discovered, err)
}

func validateAuthorizedParams(
	id ids.AccountID,
	serverConfig entities.ServerConfigReadOnly,
	token *oauth2.Token,
) error {
	if !id.Valid() {
		return ErrAccountIDIsInvalid
	}

	if serverConfig == nil {
		return ErrServerIsNil
	}

	if token == nil {
		return ErrTokenIsNil
	}

	return nil
}

func (f *connFactory) finalizeConnect(
	cancel context.CancelFunc, session *mcp.ClientSession, proto tools.Protocol, err error,
) (*asyncClient, error) {
	if err != nil {
		cancel()
		return nil, MapError(err)
	}

	return &asyncClient{
		session:      session,
		cancel:       cancel,
		usedProtocol: proto,
	}, nil
}

func (client *asyncClient) Close() error {
	client.cancel()

	if err := client.session.Close(); err != nil {
		return fmt.Errorf("closing mcp session: %w", err)
	}

	return nil
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func forbiddenChecker(next http.RoundTripper) http.RoundTripper {
	return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		resp, err := next.RoundTrip(req)
		if err != nil {
			return nil, fmt.Errorf("roundtrip failed: %w", err)
		}

		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			//nolint:errcheck,gosec // safe to ignore
			resp.Body.Close()

			return nil, ports.ErrInvalidCredentials
		}

		return resp, nil
	})
}

func authorizeHeaderCollector(next http.RoundTripper) http.RoundTripper {
	return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		resp, err := next.RoundTrip(req)
		if err != nil {
			return nil, fmt.Errorf("request failed: %w", err)
		}

		code := resp.StatusCode
		if code < http.StatusBadRequest {
			return resp, nil
		}

		//nolint:errcheck // safe to ignore
		defer resp.Body.Close()

		if code == http.StatusUnauthorized || code == http.StatusForbidden {
			return nil, extractAuthError(req.Context(), resp)
		}

		return nil, &HTTPStatusError{
			StatusCode:     resp.StatusCode,
			Status:         resp.Status,
			URL:            req.URL.String(),
			ResponseHeader: resp.Header,
		}
	})
}

func autoConnectProtocol(
	ctx context.Context, targetURL string, client *http.Client, protocol tools.Protocol,
) (*mcp.ClientSession, tools.Protocol, error) {
	switch protocol {
	case tools.ProtocolHTTP:
		session, err := connectWithTransport(ctx, &mcp.StreamableClientTransport{
			Endpoint:             targetURL,
			HTTPClient:           client,
			MaxRetries:           0,
			DisableStandaloneSSE: false,
			OAuthHandler:         nil,
		})

		return session, protocol, err

	case tools.ProtocolSSE:
		session, err := connectWithTransport(ctx, &mcp.SSEClientTransport{
			Endpoint:   targetURL,
			HTTPClient: client,
		})

		return session, protocol, err

	case tools.ProtocolUnknown: // discovery process
		return discoverProtocol(ctx, targetURL, client)
	default:
		return nil, protocol, fmt.Errorf("%w: %v", ErrUnknownProtocol, protocol)
	}
}

func discoverProtocol(
	ctx context.Context, targetURL string, client *http.Client,
) (*mcp.ClientSession, tools.Protocol, error) {
	// Try HTTP first, then SSE
	session, httpErr := connectWithTransport(ctx, &mcp.StreamableClientTransport{
		Endpoint:             targetURL,
		HTTPClient:           client,
		MaxRetries:           noRetries, // No retries - fail fast for protocol detection
		DisableStandaloneSSE: false,
		OAuthHandler:         nil,
	})
	if httpErr == nil {
		return session, tools.ProtocolHTTP, nil
	}

	session, sseErr := connectWithTransport(ctx, &mcp.SSEClientTransport{
		Endpoint:   targetURL,
		HTTPClient: client,
	})
	if sseErr == nil {
		return session, tools.ProtocolSSE, nil
	}

	return nil, tools.ProtocolUnknown, fmt.Errorf(
		"failed with both protocols: http: %w, sse: %w", httpErr, sseErr,
	)
}

// connectWithTransport attempts to connect using the specified transport.
// Returns the session on success, or an error that can be classified for fallback.
func connectWithTransport(
	ctx context.Context, transport mcp.Transport,
) (*mcp.ClientSession, error) {
	client := mcp.NewClient(clientImpl, &mcp.ClientOptions{
		KeepAlive:                     keepAliveInterval,
		Logger:                        nil, // TODO: add logger
		CreateMessageHandler:          nil,
		ElicitationHandler:            nil,
		Capabilities:                  nil,
		ElicitationCompleteHandler:    nil,
		ToolListChangedHandler:        nil,
		PromptListChangedHandler:      nil,
		ResourceListChangedHandler:    nil,
		ResourceUpdatedHandler:        nil,
		LoggingMessageHandler:         nil,
		ProgressNotificationHandler:   nil,
		CreateMessageWithToolsHandler: nil,
	})

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("connecting to mcp server: %w", err)
	}

	return session, nil
}

func newHTTPClient(transport http.RoundTripper) *http.Client {
	return &http.Client{
		Transport:     transport,
		CheckRedirect: nil,
		Jar:           nil,
		Timeout:       time.Minute,
	}
}

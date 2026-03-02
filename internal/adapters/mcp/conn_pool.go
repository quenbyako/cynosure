package mcp

import (
	"context"
	"errors"
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

type connFactory struct {
	transport      http.RoundTripper
	accountToken   AccountTokenFunc
	storage        SaveTokenFunc
	refreshTimeout time.Duration
	tracer         trace.Tracer
}

func NewConnectionFactory(
	storage SaveTokenFunc,
	accountToken AccountTokenFunc,
	tracer trace.Tracer,
) *connFactory {
	return &connFactory{
		transport:      http.DefaultTransport,
		storage:        storage,
		refreshTimeout: 10 * time.Second,
		tracer:         tracer,
	}
}

func (f *connFactory) buildAnonymousTransport() http.RoundTripper {
	return authorizeHeaderCollector(f.transport)
}

func (f *connFactory) buildTemporaryAuthorizedTransport(token *oauth2.Token) http.RoundTripper {
	// adding forbidden checker to handle invalid credentials. Since it's
	// impossible to automatically update token, we must throw it as an error.
	return forbiddenChecker(&oauth2.Transport{
		Source: oauth2.StaticTokenSource(token),
		Base:   f.transport,
	})
}

func (f *connFactory) buildAuthorizedTransport(ctx context.Context, account ids.AccountID, token *oauth2.Token, config *oauth2.Config) http.RoundTripper {
	// WithoutCancel preserves deadline but detaches from request cancellation.
	// This ensures token refresh completes even if the user cancels the request,
	// but still respects timeout constraints.
	return &oauth2.Transport{
		Source: oauth2.ReuseTokenSource(token, NewRefresher(
			// TODO: проработать момент, как втянуть сюда контекст от
			// адаптера, котрый всё закрывает, чтобы не было ситуации,
			// когда токен обновляется, но адаптер уже закрылся. это
			// конкечно решает вопрос с таймаутом, но все равно это
			// неправилыно абсолютно.
			context.WithoutCancel(ctx),
			token,
			f.storage,
			account,
			config,
			f.refreshTimeout,
		)),
		Base: f.transport,
	}
}

type asyncClient struct {
	cancel       context.CancelFunc
	session      *mcp.ClientSession
	usedProtocol tools.Protocol // Which protocol was successfully used
}

func (f *connFactory) GetAnonymous(ctx context.Context, u *url.URL, protocol tools.Protocol) (*asyncClient, error) {
	ctx, span := f.tracer.Start(ctx, "GetAnonymous", trace.WithAttributes(
		attribute.String("mcp.url", u.String()),
	))
	defer span.End()

	clientCtx, clientCancel := context.WithCancel(context.WithoutCancel(ctx))

	client := &http.Client{
		Transport: f.buildAnonymousTransport(),
	}

	session, discoveredProtocol, err := autoConnectProtocol(clientCtx, u.String(), client, protocol)
	if err != nil {
		clientCancel()
		mappedErr := MapError(err)
		span.RecordError(mappedErr)
		return nil, mappedErr
	}

	c := &asyncClient{
		session:      session,
		cancel:       clientCancel,
		usedProtocol: discoveredProtocol,
	}

	return c, nil
}

func (f *connFactory) GetPartiallyAuthorized(ctx context.Context, u *url.URL, token *oauth2.Token, protocol tools.Protocol) (*asyncClient, error) {
	ctx, span := f.tracer.Start(ctx, "GetPartiallyAuthorized", trace.WithAttributes(
		attribute.String("mcp.url", u.String()),
		attribute.Bool("mcp.token_is_nil", token == nil),
	))
	defer span.End()

	if u == nil {
		err := errors.New("url is nil")
		span.RecordError(err)
		return nil, err
	}
	if token == nil {
		err := errors.New("token is nil")
		span.RecordError(err)
		return nil, err
	}
	if !protocol.Valid() {
		err := errors.New("protocol is invalid")
		span.RecordError(err)
		return nil, err
	}

	clientCtx, clientCancel := context.WithCancel(context.WithoutCancel(ctx))

	client := &http.Client{
		Transport: f.buildTemporaryAuthorizedTransport(token),
	}

	session, discoveredProtocol, err := autoConnectProtocol(clientCtx, u.String(), client, protocol)
	if err != nil {
		clientCancel()
		mappedErr := MapError(err)
		span.RecordError(mappedErr)
		return nil, mappedErr
	}

	c := &asyncClient{
		session:      session,
		cancel:       clientCancel,
		usedProtocol: discoveredProtocol,
	}

	return c, nil
}

func (f *connFactory) GetAuthorized(ctx context.Context, accountID ids.AccountID, server entities.ServerConfigReadOnly, token *oauth2.Token) (*asyncClient, error) {
	ctx, span := f.tracer.Start(ctx, "GetAuthorized", trace.WithAttributes(
		attribute.String("mcp.account_id", accountID.ID().String()),
		attribute.Bool("mcp.token_is_nil", token == nil),
	))
	defer span.End()

	if !accountID.Valid() {
		err := errors.New("account id is invalid")
		span.RecordError(err)
		return nil, err
	}
	if server == nil {
		err := errors.New("server is nil")
		span.RecordError(err)
		return nil, err
	}
	if token == nil {
		err := errors.New("token is nil")
		span.RecordError(err)
		return nil, err
	}

	clientCtx, clientCancel := context.WithCancel(context.WithoutCancel(ctx))

	client := &http.Client{
		Transport: f.buildAuthorizedTransport(clientCtx, accountID, token, server.AuthConfig()),
	}

	session, discoveredProtocol, err := autoConnectProtocol(clientCtx, server.SSELink().String(), client, server.PreferredProtocol())
	if err != nil {
		clientCancel()
		mappedErr := MapError(err)
		span.RecordError(mappedErr)
		return nil, mappedErr
	}

	c := &asyncClient{
		session:      session,
		cancel:       clientCancel,
		usedProtocol: discoveredProtocol,
	}

	return c, nil
}

func (c *asyncClient) Close() error {
	c.cancel()

	return c.session.Close()
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func forbiddenChecker(next http.RoundTripper) http.RoundTripper {
	return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		resp, err := next.RoundTrip(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
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
			return nil, err
		}

		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			resp.Body.Close()
			return nil, extractAuthError(resp)
		}

		if resp.StatusCode >= 400 {
			resp.Body.Close()
			// We return an error BUT we want to preserve headers for Auth errors (401/403)
			// or protocol fallback decisions (400/404/405).
			return nil, &HTTPStatusError{
				StatusCode:     resp.StatusCode,
				Status:         resp.Status,
				URL:            req.URL.String(),
				ResponseHeader: resp.Header,
			}
		}

		return resp, nil
	})
}

func autoConnectProtocol(ctx context.Context, url string, client *http.Client, protocol tools.Protocol) (*mcp.ClientSession, tools.Protocol, error) {
	switch protocol {
	case tools.ProtocolHTTP:
		session, err := connectWithTransport(ctx, &mcp.StreamableClientTransport{
			Endpoint:   url,
			HTTPClient: client,
		})
		return session, protocol, err

	case tools.ProtocolSSE:
		session, err := connectWithTransport(ctx, &mcp.SSEClientTransport{
			Endpoint:   url,
			HTTPClient: client,
		})

		return session, protocol, err

	case tools.ProtocolUnknown: // discovery process
		// Try HTTP first, then SSE
		session, httpErr := connectWithTransport(ctx, &mcp.StreamableClientTransport{
			Endpoint:   url,
			HTTPClient: client,
			MaxRetries: -1, // No retries - fail fast for protocol detection
		})
		if httpErr == nil {
			return session, tools.ProtocolHTTP, nil
		}

		session, sseErr := connectWithTransport(ctx, &mcp.SSEClientTransport{
			Endpoint:   url,
			HTTPClient: client,
		})
		if sseErr == nil {
			return session, tools.ProtocolSSE, nil
		}

		return nil, protocol, fmt.Errorf("failed to connect with both protocols: http: %w, sse: %w", httpErr, sseErr)
	default:
		return nil, protocol, fmt.Errorf("unknown protocol: %v", protocol)
	}
}

// connectWithTransport attempts to connect using the specified transport.
// Returns the session on success, or an error that can be classified for fallback.
func connectWithTransport(ctx context.Context, transport mcp.Transport) (*mcp.ClientSession, error) {
	client := mcp.NewClient(clientImpl, &mcp.ClientOptions{
		KeepAlive: 1 * time.Second,
	})

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, err
	}

	return session, nil
}

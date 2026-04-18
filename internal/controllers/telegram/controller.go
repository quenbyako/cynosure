// Package telegram implements Telegram controller.
package telegram

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	botapi "github.com/quenbyako/cynosure/contrib/tg-openapi/gen/go/botapi"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"
	noopTrace "go.opentelemetry.io/otel/trace/noop"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/chat"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/users"
)

const (
	pkgName = "github.com/quenbyako/cynosure/internal/controllers/telegram"
)

type Handler struct {
	// TODO: maybe it's not the best pattern? Maybe channels or mutexes are better? idk
	//nolint:containedctx // lifecycleCtx provides context for handler worker.
	lifecycleCtx   context.Context
	log            LogCallbacks
	tracer         trace.Tracer
	srv            *chat.Usecase
	users          *users.Usecase
	client         *botapi.ClientWithResponses
	updateInterval time.Duration
}

var _ botapi.StrictWebhookInterface = (*Handler)(nil)

type newParams struct {
	log            LogCallbacks
	tracer         trace.TracerProvider
	updateInterval time.Duration
}

func buildNewParams(opts ...NewOption) newParams {
	params := newParams{
		updateInterval: defaultUpdateInterval,
		log:            NoOpLogCallbacks{},
		tracer:         noopTrace.NewTracerProvider(),
	}

	for _, opt := range opts {
		opt(&params)
	}

	return params
}

type NewOption func(*newParams)

func WithUpdateInterval(interval time.Duration) NewOption {
	return func(h *newParams) { h.updateInterval = interval }
}

func WithLogCallbacks(log LogCallbacks) NewOption {
	return func(h *newParams) { h.log = log }
}

func WithTracer(tracer trace.TracerProvider) NewOption {
	return func(h *newParams) { h.tracer = tracer }
}

func (p *newParams) httpClient() *http.Client {
	return &http.Client{
		Transport: otelhttp.NewTransport(
			http.DefaultTransport,
			otelhttp.WithTracerProvider(p.tracer),
		),
		Timeout:       0,
		CheckRedirect: nil,
		Jar:           nil,
	}
}

const (
	defaultUpdateInterval = 2 * time.Second
)

func New(
	ctx context.Context, srv *chat.Usecase, usecase *users.Usecase,
	serverPublicAddress *url.URL, token []byte, opts ...NewOption,
) (http.Handler, error) {
	params := buildNewParams(opts...)

	client, err := newClient(token, &params)
	if err != nil {
		return nil, err
	}

	hookSecret, err := setWebhook(ctx, client, serverPublicAddress)
	if err != nil {
		return nil, err
	}

	handler := newHandler(ctx, srv, usecase, client, &params)
	inner := botapi.NewStrictWebhookHandler(handler, []botapi.StrictMiddlewareFunc{})

	return wrapWebhookHandler(inner, hookSecret), nil
}

func newHandler(
	ctx context.Context, chatUsecase *chat.Usecase, usersUsecase *users.Usecase,
	client *botapi.ClientWithResponses, params *newParams,
) *Handler {
	return &Handler{
		log:            params.log,
		tracer:         params.tracer.Tracer(pkgName),
		srv:            chatUsecase,
		users:          usersUsecase,
		client:         client,
		updateInterval: params.updateInterval,
		lifecycleCtx:   ctx,
	}
}

func wrapWebhookHandler(inner botapi.WebhookInterface, secret string) http.Handler {
	return botapi.WebhookHandlerWithOptions(inner, botapi.WebhookServerOptions{
		BaseRouter:       nil,
		ErrorHandlerFunc: nil,
		BaseURL:          "",
		Middlewares: []botapi.MiddlewareFunc{
			botapi.AuthenticateWebhook(secret),
		},
	})
}

func newClient(token []byte, params *newParams) (*botapi.ClientWithResponses, error) {
	client, err := botapi.NewClientWithResponses("https://api.telegram.org/bot"+string(token),
		botapi.WithHTTPClient(params.httpClient()),
	)
	if err != nil {
		return nil, fmt.Errorf("creating telegram client: %w", err)
	}

	return client, nil
}

type ArgumentError string

func (e ArgumentError) Error() string {
	return string(e)
}

const (
	errAddressIsNil ArgumentError = "server public address is nil"
)

func setWebhook(
	ctx context.Context, client *botapi.ClientWithResponses, addr *url.URL,
) (string, error) {
	if addr == nil {
		return "", errAddressIsNil
	}

	token, err := botapi.MakeSecretToken()
	if err != nil {
		return "", fmt.Errorf("generating secret token: %w", err)
	}

	resp, err := client.SetWebhookWithResponse(ctx, botapi.SetWebhookJSONRequestBody{
		Url:                addr.String(),
		AllowedUpdates:     nil,
		DropPendingUpdates: nil,
		IpAddress:          nil,
		MaxConnections:     nil,
		SecretToken:        &token,
	})
	if err != nil {
		return "", fmt.Errorf("setting telegram webhook: %w", err)
	}

	if resp.JSON200 == nil || (!resp.JSON200.Ok || !resp.JSON200.Result) {
		return "", errAPI("setting telegram webhook", resp.Status(), resp.JSONDefault)
	}

	return token, nil
}

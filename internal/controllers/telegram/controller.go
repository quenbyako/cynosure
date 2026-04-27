// Package telegram implements Telegram controller.
package telegram

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/quenbyako/cynosure/contrib/taskpool"
	botapi "github.com/quenbyako/cynosure/contrib/tg-openapi/gen/go/botapi"
	"go.opentelemetry.io/otel/trace"
	noopTrace "go.opentelemetry.io/otel/trace/noop"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/chat"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/users"
)

const (
	pkgName = "github.com/quenbyako/cynosure/internal/controllers/telegram"

	defaultUpdateInterval = 2 * time.Second
	defaultMaxWorkers     = 10
)

type Handler struct {
	log            LogCallbacks
	tracer         trace.Tracer
	srv            *chat.Usecase
	users          *users.Usecase
	client         *botapi.ClientWithResponses
	pool           *taskpool.TaskPool[asyncProcessRequest]
	updateInterval time.Duration
}

var _ botapi.StrictWebhookInterface = (*Handler)(nil)

type newParams struct {
	log            LogCallbacks
	tracer         trace.TracerProvider
	client         http.RoundTripper
	updateInterval time.Duration
	maxWorkers     int
}

func buildNewParams(opts ...NewOption) newParams {
	params := newParams{
		updateInterval: defaultUpdateInterval,
		log:            NoOpLogCallbacks{},
		tracer:         noopTrace.NewTracerProvider(),
		client:         http.DefaultTransport,
		maxWorkers:     defaultMaxWorkers,
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

func WithClient(client http.RoundTripper) NewOption {
	return func(h *newParams) { h.client = client }
}

func WithLogCallbacks(log LogCallbacks) NewOption {
	return func(h *newParams) { h.log = log }
}

func WithTracer(tracer trace.TracerProvider) NewOption {
	return func(h *newParams) { h.tracer = tracer }
}

func WithMaxWorkers(maxWorkers int) NewOption {
	return func(h *newParams) { h.maxWorkers = maxWorkers }
}

func New(
	ctx context.Context, srv *chat.Usecase, usecase *users.Usecase,
	serverPublicAddress *url.URL, token []byte, opts ...NewOption,
) (http.Handler, func(context.Context) error, error) {
	params := buildNewParams(opts...)

	client, err := newClient(token, &params)
	if err != nil {
		return nil, nil, err
	}

	hookSecret, err := setWebhook(ctx, client, serverPublicAddress)
	if err != nil {
		return nil, nil, err
	}

	handler := newHandler(srv, usecase, client, &params)
	inner := botapi.NewStrictWebhookHandler(handler, nil)

	return wrapWebhookHandler(inner, hookSecret), handler.Run, nil
}

func newHandler(
	chatUsecase *chat.Usecase, usersUsecase *users.Usecase,
	client *botapi.ClientWithResponses, params *newParams,
) *Handler {
	handler := Handler{
		log:            params.log,
		tracer:         params.tracer.Tracer(pkgName),
		srv:            chatUsecase,
		users:          usersUsecase,
		client:         client,
		updateInterval: params.updateInterval,
		pool:           nil,
	}

	handler.pool = taskpool.New(params.maxWorkers, handler.asyncProcess)

	return &handler
}

// Run starts the handler and blocks until the context is canceled or the
// handler fails.
func (h *Handler) Run(ctx context.Context) error {
	if err := h.pool.Run(ctx); err != nil {
		return fmt.Errorf("running telegram controller task pool: %w", err)
	}

	return nil
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
		botapi.WithHTTPClient(&http.Client{
			Transport:     params.client,
			CheckRedirect: nil,
			Jar:           nil,
			Timeout:       0,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("creating telegram client: %w", err)
	}

	return client, nil
}

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

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

const (
	defaultUpdateInterval = 2 * time.Second
)

func New(
	ctx context.Context, srv *chat.Usecase, usecase *users.Usecase,
	serverPublicAddress *url.URL, token []byte, opts ...NewOption,
) (http.Handler, error) {
	params := newParams{
		updateInterval: defaultUpdateInterval,
		log:            NoOpLogCallbacks{},
		tracer:         noopTrace.NewTracerProvider(),
	}
	for _, opt := range opts {
		opt(&params)
	}

	client, err := botapi.NewClientWithResponses("https://api.telegram.org/bot"+string(token),
		botapi.WithHTTPClient(&http.Client{
			Transport: otelhttp.NewTransport(
				http.DefaultTransport,
				otelhttp.WithTracerProvider(params.tracer),
			),
			Timeout:       0,
			CheckRedirect: nil,
			Jar:           nil,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("creating telegram client: %w", err)
	}

	resp, err := client.SetWebhookWithResponse(ctx, botapi.SetWebhookJSONRequestBody{
		Url:                serverPublicAddress.String(),
		AllowedUpdates:     nil,
		DropPendingUpdates: nil,
		IpAddress:          nil,
		MaxConnections:     nil,
		SecretToken:        nil,
	})
	if err != nil {
		return nil, fmt.Errorf("setting telegram webhook: %w", err)
	}

	if resp.JSON200 == nil || (!resp.JSON200.Ok || !resp.JSON200.Result) {
		return nil, fmt.Errorf("failed to set telegram webhook: %s", resp.Status())
	}

	handler := &Handler{
		log:            params.log,
		tracer:         params.tracer.Tracer(pkgName),
		srv:            srv,
		users:          usecase,
		client:         client,
		updateInterval: params.updateInterval,
		lifecycleCtx:   ctx,
	}

	inner := botapi.NewStrictWebhookHandler(handler, []botapi.StrictMiddlewareFunc{})

	return botapi.WebhookHandler(inner), nil
}

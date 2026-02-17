package telegram

import (
	"context"
	"fmt"
	"net/http"
	"time"

	botapi "github.com/quenbyako/cynosure/contrib/tg-openapi/gen/go/botapi"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"
	noopTrace "go.opentelemetry.io/otel/trace/noop"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/chat"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/users"
)

const pkgName = "github.com/quenbyako/cynosure/internal/controllers/telegram"

type Handler struct {
	srv            *chat.Usecase
	users          *users.Usecase
	client         *botapi.ClientWithResponses
	updateInterval time.Duration
	lifecycleCtx   context.Context

	log    LogCallbacks
	tracer trace.Tracer
}

var _ botapi.StrictWebhookInterface = (*Handler)(nil)

type newParams struct {
	updateInterval time.Duration
	log            LogCallbacks
	tracer         trace.TracerProvider
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

func New(ctx context.Context, srv *chat.Usecase, users *users.Usecase, serverPublicAddress string, token []byte, opts ...NewOption) (http.Handler, error) {
	p := newParams{
		updateInterval: time.Second * 2,
		log:            NoOpLogCallbacks{},
		tracer:         noopTrace.NewTracerProvider(),
	}
	for _, opt := range opts {
		opt(&p)
	}

	client, err := botapi.NewClientWithResponses("https://api.telegram.org/bot"+string(token),
		botapi.WithHTTPClient(&http.Client{
			Transport: otelhttp.NewTransport(http.DefaultTransport, otelhttp.WithTracerProvider(p.tracer)),
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("creating telegram client: %w", err)
	}

	resp, err := client.SetWebhookWithResponse(ctx, botapi.SetWebhookJSONRequestBody{
		Url: serverPublicAddress,
	})
	if err != nil {
		return nil, fmt.Errorf("setting telegram webhook: %w", err)
	}
	if resp.JSON200 == nil || !(resp.JSON200.Ok && resp.JSON200.Result) {
		return nil, fmt.Errorf("failed to set telegram webhook: %s", resp.Status())
	}

	h := &Handler{
		log:            p.log,
		tracer:         p.tracer.Tracer(pkgName),
		srv:            srv,
		users:          users,
		client:         client,
		updateInterval: p.updateInterval,
		lifecycleCtx:   ctx,
	}

	inner := botapi.NewStrictWebhookHandler(h, []botapi.StrictMiddlewareFunc{})

	return botapi.WebhookHandler(inner), nil
}

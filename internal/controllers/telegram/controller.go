package telegram

import (
	"context"
	"net/http"
	"time"

	botapi "github.com/quenbyako/cynosure/contrib/tg-openapi/gen/go/botapi"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/chat"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/usecases/users"
)

type Handler struct {
	log LogCallbacks

	srv            *chat.Usecase
	users          *users.Usecase
	client         *botapi.ClientWithResponses
	updateInterval time.Duration
	lifecycleCtx   context.Context
}

var _ botapi.StrictWebhookInterface = (*Handler)(nil)

type HandlerOption func(*Handler)

func WithUpdateInterval(interval time.Duration) HandlerOption {
	return func(h *Handler) { h.updateInterval = interval }
}

func WithLogCallbacks(log LogCallbacks) HandlerOption {
	return func(h *Handler) { h.log = log }
}

func NewHandler(ctx context.Context, srv *chat.Usecase, users *users.Usecase, serverPublicAddress string, token []byte, opts ...HandlerOption) http.Handler {
	client, err := botapi.NewClientWithResponses("https://api.telegram.org/bot" + string(token))
	if err != nil {
		panic(err)
	}

	resp, err := client.SetWebhookWithResponse(ctx, botapi.SetWebhookJSONRequestBody{
		Url: serverPublicAddress,
	})
	if err != nil {
		panic(err)
	}
	if resp.JSON200 == nil || !(resp.JSON200.Ok && resp.JSON200.Result) {
		panic("failed to set webhook: " + resp.Status())
	}

	h := &Handler{
		log: NoOpLogCallbacks{},

		srv:            srv,
		users:          users,
		client:         client,
		updateInterval: time.Second * 2, // Default to 2 seconds, should be made configurable
		lifecycleCtx:   ctx,
	}

	for _, opt := range opts {
		opt(h)
	}

	inner := botapi.NewStrictWebhookHandler(h, []botapi.StrictMiddlewareFunc{})

	return botapi.WebhookHandler(inner)
}

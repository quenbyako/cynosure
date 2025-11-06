package gateway

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/quenbyako/core"
	"github.com/quenbyako/cynosure/contrib/onelog"

	"github.com/quenbyako/cynosure/internal/controllers/tgbot"
	"github.com/quenbyako/cynosure/internal/domains/gateway/usecases"
)

type SecretGetter interface {
	Get(ctx context.Context) ([]byte, error)
}

type appParams struct {
	telegramToken   SecretGetter
	webhookRegistar func(http.Handler)
	webhookAddr     *url.URL

	a2aClient *url.URL

	observability core.Metrics
}

func WithTelegramToken(token SecretGetter) AppOpts {
	return func(p *appParams) { p.telegramToken = token }
}

func WithWebhookPort(f func(http.Handler)) AppOpts {
	return func(p *appParams) { p.webhookRegistar = f }
}

func WithWebhookAddress(addr url.URL) AppOpts {
	return func(p *appParams) { p.webhookAddr = &addr }
}

func WithA2AClientAddress(addr url.URL) AppOpts {
	return func(p *appParams) { p.a2aClient = &addr }
}

func (p *appParams) validate() error {
	var errs []error

	if p.webhookRegistar == nil {
		errs = append(errs, errors.New("webhook registar is required"))
	}
	if p.telegramToken == nil {
		errs = append(errs, errors.New("telegram token is required"))
	}
	if p.webhookAddr == nil {
		errs = append(errs, errors.New("webhook address is required"))
	}
	if p.a2aClient == nil {
		errs = append(errs, errors.New("a2a client address is required"))
	}

	return errors.Join(errs...)
}

type AppOpts func(*appParams)

func WithObservability(observability core.Metrics) AppOpts {
	return func(p *appParams) { p.observability = observability }
}

func NewApp(ctx context.Context, opts ...AppOpts) *App {
	p := appParams{
		observability: core.NoopMetrics(),
	}
	for _, opt := range opts {
		opt(&p)
	}
	if err := p.validate(); err != nil {
		panic(err)
	}

	return must(buildApp(ctx, &p))
}

func newApp(
	p *appParams,
	usecase *usecases.Usecase,
) (*App, error) {
	p.webhookRegistar(tgbot.NewHandler(usecase))

	return &App{
		log: onelog.Wrap(p.observability),
	}, nil
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

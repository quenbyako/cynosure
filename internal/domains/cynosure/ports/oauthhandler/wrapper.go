package oauthhandler

import (
	"context"
	"net/url"
	"time"

	"golang.org/x/oauth2"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
)

type PortWrapped interface {
	Port

	_PortWrapped()
}

type portWrapped struct {
	w Port
	t *observable
}

func (t *portWrapped) _PortWrapped() {}

func Wrap(client Port, observable ports.ObserveStack) PortWrapped {
	if observable == nil {
		observable = ports.NoOpObserveStack()
	}

	t := portWrapped{
		w: client,
		t: newObservable(observable),
	}

	return &t
}

func (t *portWrapped) RegisterClient(
	ctx context.Context,
	resourceURL *url.URL,
	clientName string,
	setRedirect *url.URL,
	opts ...RegisterClientOption,
) (cfg *oauth2.Config, expiresAt time.Time, err error) {
	ctx, span := t.t.registerClient(ctx, resourceURL.String(), clientName)
	defer span.end()

	cfg, expiresAt, err = t.w.RegisterClient(ctx, resourceURL, clientName, setRedirect, opts...)
	span.recordError(err)

	//nolint:wrapcheck // should not wrap adapter errors
	return cfg, expiresAt, err
}

func (t *portWrapped) RefreshToken(
	ctx context.Context, config *oauth2.Config, token *oauth2.Token,
) (res *oauth2.Token, err error) {
	ctx, span := t.t.refreshToken(ctx, config.ClientID, config.Endpoint.AuthURL)
	defer span.end()

	res, err = t.w.RefreshToken(ctx, config, token)
	span.recordError(err)

	//nolint:wrapcheck // should not wrap adapter errors
	return res, err
}

func (t *portWrapped) Exchange(
	ctx context.Context, config *oauth2.Config, code string, verifier []byte,
) (res *oauth2.Token, err error) {
	ctx, span := t.t.exchange(ctx, config.ClientID, config.Endpoint.TokenURL)
	defer span.end()

	res, err = t.w.Exchange(ctx, config, code, verifier)
	span.recordError(err)

	//nolint:wrapcheck // should not wrap adapter errors
	return res, err
}

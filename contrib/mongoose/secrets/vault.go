package secrets

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/api/auth/cert"
)

type VaultStorage struct {
	conn *api.Client
}

type vaultParams struct {
	client http.RoundTripper
}

type NewVaultOption func(*vaultParams)

func WithVaultHTTPClient(client http.RoundTripper) NewVaultOption {
	return func(p *vaultParams) { p.client = client }
}

func NewVault(ctx context.Context, u *url.URL, opts ...NewVaultOption) (Storage, error) {
	if u == nil {
		return nil, errors.New("no URL provided")
	}

	p := vaultParams{
		client: http.DefaultTransport,
	}
	for _, o := range opts {
		o(&p)
	}

	client, err := api.NewClient(buildConfig(p.client, u))
	if err != nil {
		return nil, err
	}

	auth, err := cert.NewCertAuth()
	if err != nil {
		return nil, err
	}

	token, err := client.Auth().Login(ctx, auth)
	if err != nil {
		return nil, err
	}

	client.SetToken(token.Auth.ClientToken)

	return &VaultStorage{
		conn: client,
	}, nil
}

func (c *VaultStorage) GetSecret(ctx context.Context, key string) (Secret, error) {
	s := &cachedSecret{
		storage: c,
		key:     key,
	}

	_, err := s.Get(ctx)
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (*VaultStorage) Close() error { return nil }

type cachedSecret struct {
	storage   *VaultStorage
	mountPath string
	dataKey   string
	key       string

	cachedValue []byte
	lastUpdated time.Time
}

var _ Secret = (*cachedSecret)(nil)

// Get implements Secret.
func (c *cachedSecret) Get(ctx context.Context) ([]byte, error) {
	if time.Since(c.lastUpdated) < time.Minute && c.cachedValue != nil {
		return c.cachedValue, nil
	}

	secret, err := c.storage.conn.KVv2(c.mountPath).Get(ctx, c.key)
	if err != nil {
		return nil, err
	}

	res, ok := secret.Data[c.dataKey]
	if !ok {
		return nil, ErrSecretNotFound
	}
	str, ok := res.(string)
	if !ok {
		return nil, errors.New("secret is not a string")
	}

	c.cachedValue = []byte(str)
	c.lastUpdated = time.Now()

	return c.cachedValue, nil

}

func buildConfig(transport http.RoundTripper, addr *url.URL) *api.Config {
	config := &api.Config{
		Address: addr.String(),
		HttpClient: &http.Client{
			Transport: transport,
			// Ensure redirects are not automatically followed
			// Note that this is sane for the API client as it has its own
			// redirect handling logic (and thus also for command/meta),
			// but in e.g. http_test actual redirect handling is necessary
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Returning this value causes the Go net library to not close the
				// response body and to nil out the error. Otherwise retry clients may
				// try three times on every redirect because it sees an error from this
				// function (to prevent redirects) passing through to it.
				return http.ErrUseLastResponse
			},
		},
		Timeout:      time.Second * 60,
		MinRetryWait: time.Millisecond * 1000,
		MaxRetryWait: time.Millisecond * 1500,
		MaxRetries:   2,
		Backoff:      retryablehttp.RateLimitLinearJitterBackoff,
	}

	return config
}

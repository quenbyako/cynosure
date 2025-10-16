package goose

import (
	"context"
	"errors"
	"fmt"

	"github.com/caarlos0/env/v11"
	"github.com/quenbyako/cynosure/contrib/mongoose/secrets"
)

type rawSecret struct {
	path string

	wrapped secrets.Secret
}

var _ secrets.Secret = (*rawSecret)(nil)
var _ configureType = (*rawSecret)(nil)

func parseSecret(c *[]configureType) env.ParserFunc {
	return func(v string) (any, error) {
		w := &rawSecret{path: v}
		*c = append(*c, w)
		return w, nil
	}
}

func (s *rawSecret) Get(ctx context.Context) ([]byte, error) {
	if s.wrapped == nil {
		panic("uninitialized")
	}

	return s.wrapped.Get(ctx)
}

func (s *rawSecret) configure(ctx context.Context, data *configureData) error {
	if data.secretEngine == nil {
		return errors.New("no secret engine provided")
	}

	secret, err := data.secretEngine.GetSecret(ctx, s.path)
	if err != nil {
		return fmt.Errorf("getting secret %q: %w", s.path, err)
	}

	s.wrapped = secret
	return nil
}

func (s *rawSecret) acquire(ctx context.Context, data *acquireData) error   { return nil }
func (s *rawSecret) shutdown(ctx context.Context, data *shutdownData) error { return nil }

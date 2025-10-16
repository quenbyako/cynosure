package secrets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync/atomic"

	"github.com/vincent-petithory/dataurl"
)

var (
	ErrSecretNotFound = errors.New("secret not found")
	ErrSecretNotSet   = errors.New("secret not set")
)

type Secret interface {
	Get(ctx context.Context) ([]byte, error)
}

type Storage interface {
	io.Closer

	// context is used only for checking that this key is exists, and for first
	// initialization.
	GetSecret(ctx context.Context, key string) (Secret, error)
}

func newSecretStorage(ctx context.Context, u *url.URL) (Storage, error) {
	switch scheme := u.Scheme; scheme {
	case "file":
		path := u.Host
		if u.Path != "" {
			path += u.Path
		}

		return NewFile(path)
	case "vault":
		return NewVault(ctx, u)
	default:
		return nil, fmt.Errorf("unsupported storage type: %q", scheme)
	}
}

type SecretEngine struct {
	closed atomic.Bool

	storages map[string]Storage
}

func BuildSecretEngine(ctx context.Context, u map[string]*url.URL) (*SecretEngine, error) {
	if len(u) == 0 {
		return &SecretEngine{}, nil
	}

	storages := make(map[string]Storage, len(u))
	for scheme, url := range u {
		if url == nil {
			// urls MIGHT be nil, cause user doesn't call them each time.
			//
			// However, we should throw an error that we know about this type,
			// but user just didn't provide it.
			storages[scheme] = &unsetStorage{name: scheme}
			continue
		}

		storage, err := newSecretStorage(ctx, url)
		if err != nil {
			return &SecretEngine{}, fmt.Errorf("creating storage for scheme %q: %w", scheme, err)
		}
		storages[scheme] = storage
	}
	return &SecretEngine{storages: storages}, nil
}

func (e *SecretEngine) GetSecret(ctx context.Context, addr string) (Secret, error) {
	if e.closed.Load() {
		return nil, io.ErrClosedPipe
	}

	if addr == "" {
		return &emptySecret{}, nil
	}

	// data is not correct url scheme, cause usually we are using data:// or
	// something like this.
	//
	// Still, we had to check it in that way.
	if strings.HasPrefix(addr, "data:") {
		data, err := dataurl.DecodeString(addr)
		if err != nil {
			return nil, fmt.Errorf("decoding data URL: %w", err)
		}

		return &plainSecret{data: data.Data}, nil
	}

	key, err := url.Parse(addr)
	if err != nil {
		return nil, fmt.Errorf("parsing secret URL %q: %w", addr, err)
	}

	storage, ok := e.storages[key.Scheme]
	if !ok {
		return nil, fmt.Errorf("no storage for scheme %q", key.Scheme)
	}

	secret, err := storage.GetSecret(ctx, key.Opaque)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret from storage: %w", err)
	}

	return secret, nil
}

func (e *SecretEngine) Close() error {
	if e.closed.CompareAndSwap(false, true) {
		return nil
	}

	var errs []error
	for _, s := range e.storages {
		if err := s.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

type emptySecret struct{}

func (s *emptySecret) Get(context.Context) ([]byte, error) { return nil, nil }

type plainSecret struct {
	data []byte
}

func (s *plainSecret) Get(context.Context) ([]byte, error) { return s.data, nil }

type unsetStorage struct {
	name string
}

var _ Storage = (*unsetStorage)(nil)

func (u *unsetStorage) GetSecret(ctx context.Context, key string) (Secret, error) {
	return nil, fmt.Errorf("storage %q is unset", u.name)
}

func (u *unsetStorage) Close() error {
	return nil
}

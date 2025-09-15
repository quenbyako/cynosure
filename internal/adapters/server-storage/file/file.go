package file

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sync"
	"time"

	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v3"

	"tg-helper/internal/domains/components/ids"
	"tg-helper/internal/domains/ports"
)

const pkgName = "internal/adapters/server-storage/file"

type FileServerStorage struct {
	mu   sync.Mutex
	path string

	trace trace.Tracer
}

var _ ports.ServerStorage = (*FileServerStorage)(nil)

type newParams struct {
	trace trace.TracerProvider
}

type NewOption func(*newParams)

func WithTracer(tracer trace.TracerProvider) NewOption {
	return func(p *newParams) { p.trace = tracer }
}

func New(path string, opts ...NewOption) *FileServerStorage {
	p := newParams{
		trace: noop.NewTracerProvider(),
	}
	for _, opt := range opts {
		opt(&p)
	}

	return &FileServerStorage{
		path:  path,
		trace: p.trace.Tracer(pkgName),
	}
}

func (f *FileServerStorage) AddServer(ctx context.Context, server ids.ServerID, info ports.ServerInfo) error {
	ctx, span := f.trace.Start(ctx, "FileServerStorage.AddServer")
	defer span.End()

	f.mu.Lock()
	defer f.mu.Unlock()

	data, err := os.ReadFile(f.path)
	if errors.Is(err, os.ErrNotExist) {
		data = []byte{}

	} else if err != nil {
		return fmt.Errorf("failed to read token storage file: %w", err)
	}

	if len(data) == 0 {
		data = []byte("{}")
	}

	var storage storageSchema
	if err := yaml.Unmarshal(data, &storage); err != nil {
		return fmt.Errorf("failed to parse token storage file: %w", err)
	}

	storage[server.ID().String()] = serverInfo{
		Link:       info.SSELink.String(),
		Config:     info.AuthConfig,
		Expiration: info.ConfigExpiration,
	}

	buf := bytes.NewBuffer(nil)
	enc := yaml.NewEncoder(buf)
	enc.SetIndent(2)

	if err := enc.Encode(storage); err != nil {
		return fmt.Errorf("failed to marshal updated token storage: %w", err)
	}

	if err := os.WriteFile(f.path, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write updated token storage file: %w", err)
	}

	return nil
}

func (f *FileServerStorage) GetServerInfo(ctx context.Context, server ids.ServerID) (*ports.ServerInfo, error) {
	ctx, span := f.trace.Start(ctx, "FileServerStorage.GetServerInfo")
	defer span.End()

	f.mu.Lock()
	defer f.mu.Unlock()

	data, err := os.ReadFile(f.path)
	if os.IsNotExist(err) {
		return nil, ports.ErrNotFound
	} else if err != nil {
		return nil, fmt.Errorf("failed to read token storage file: %w", err)
	}

	if len(data) == 0 {
		return nil, ports.ErrNotFound
	}

	var storage storageSchema
	if err := yaml.Unmarshal(data, &storage); err != nil {
		return nil, fmt.Errorf("failed to parse token storage file: %w", err)
	}

	cfg, exists := storage[server.ID().String()]
	if !exists {
		return nil, ports.ErrNotFound
	}

	return &ports.ServerInfo{
		SSELink:          must(url.Parse(cfg.Link)),
		AuthConfig:       cfg.Config,
		ConfigExpiration: cfg.Expiration,
	}, nil
}

func (f *FileServerStorage) ListServers(ctx context.Context, limit uint, token string) (m map[ids.ServerID]ports.ServerInfo, nextToken string, err error) {
	ctx, span := f.trace.Start(ctx, "FileServerStorage.ListServers")
	defer span.End()

	f.mu.Lock()
	defer f.mu.Unlock()

	data, err := os.ReadFile(f.path)
	if os.IsNotExist(err) {
		return nil, "", nil

	} else if err != nil {
		return nil, "", fmt.Errorf("failed to read token storage file: %w", err)
	}

	if len(data) == 0 {
		return nil, "", nil
	}

	var storage storageSchema
	if err := yaml.Unmarshal(data, &storage); err != nil {
		return nil, "", fmt.Errorf("failed to parse token storage file: %w", err)
	}

	servers := make(map[ids.ServerID]ports.ServerInfo, len(storage))
	for name, info := range storage {
		servers[must(ids.NewServerIDFromString(name))] = ports.ServerInfo{
			SSELink:          must(url.Parse(info.Link)),
			AuthConfig:       info.Config,
			ConfigExpiration: info.Expiration,
		}
	}

	return servers, "", nil
}

func (f *FileServerStorage) LookupByURL(ctx context.Context, url *url.URL) (ids.ServerID, ports.ServerInfo, error) {
	ctx, span := f.trace.Start(ctx, "FileServerStorage.LookupByURL")
	defer span.End()

	f.mu.Lock()
	defer f.mu.Unlock()

	data, err := os.ReadFile(f.path)
	if os.IsNotExist(err) {
		return ids.ServerID{}, ports.ServerInfo{}, ports.ErrNotFound
	} else if err != nil {
		return ids.ServerID{}, ports.ServerInfo{}, fmt.Errorf("failed to read token storage file: %w", err)
	}

	if len(data) == 0 {
		return ids.ServerID{}, ports.ServerInfo{}, ports.ErrNotFound
	}

	var storage storageSchema
	if err := yaml.Unmarshal(data, &storage); err != nil {
		return ids.ServerID{}, ports.ServerInfo{}, fmt.Errorf("failed to parse token storage file: %w", err)
	}

	urlStr := url.String()

	for name, info := range storage {
		if info.Link == urlStr {
			return must(ids.NewServerIDFromString(name)), ports.ServerInfo{
				SSELink:          must(url.Parse(info.Link)),
				AuthConfig:       info.Config,
				ConfigExpiration: info.Expiration,
			}, nil
		}
	}

	return ids.ServerID{}, ports.ServerInfo{}, ports.ErrNotFound
}

type storageSchema map[serverID]serverInfo

type serverID = string

type serverInfo struct {
	Link       string         `yaml:"url"`
	Config     *oauth2.Config `yaml:"config"`
	Expiration time.Time      `yaml:"expiration,omitempty"`
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

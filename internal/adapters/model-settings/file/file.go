package file

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"sync"

	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"gopkg.in/yaml.v3"

	"tg-helper/internal/domains/components/ids"
	"tg-helper/internal/domains/entities"
	"tg-helper/internal/domains/ports"
)

const pkgName = "internal/adapters/model-settings/file"

type File struct {
	mu   sync.Mutex
	path string

	trace trace.Tracer
}

var _ ports.ModelSettingsStorage = (*File)(nil)

type newParams struct {
	trace trace.TracerProvider
}

type NewOption func(*newParams)

func WithTracer(tracer trace.TracerProvider) NewOption {
	return func(p *newParams) { p.trace = tracer }
}

func New(path string, opts ...NewOption) *File {
	p := newParams{
		trace: noop.NewTracerProvider(),
	}
	for _, opt := range opts {
		opt(&p)
	}

	return &File{
		path:  path,
		trace: p.trace.Tracer(pkgName),
	}
}

// DeleteModel implements ports.ModelSettingsStorage.
func (f *File) DeleteModel(ctx context.Context, model ids.ModelConfigID) error {
	ctx, span := f.trace.Start(ctx, "File.DeleteModel")
	defer span.End()

	f.mu.Lock()
	defer f.mu.Unlock()

	data, err := os.ReadFile(f.path)
	if os.IsNotExist(err) {
		return ports.ErrNotFound
	} else if err != nil {
		return fmt.Errorf("failed to read token storage file: %w", err)
	}

	if len(data) == 0 {
		return ports.ErrNotFound
	}

	var storage storageSchema
	if err := yaml.Unmarshal(data, &storage); err != nil {
		return fmt.Errorf("failed to parse token storage file: %w", err)
	}

	delete(storage, model.ID().String())

	data, err = yaml.Marshal(storage)
	if err != nil {
		return fmt.Errorf("failed to marshal token storage file: %w", err)
	}

	if err := os.WriteFile(f.path, data, 0644); err != nil {
		return fmt.Errorf("failed to write token storage file: %w", err)
	}

	return nil
}

// GetModel implements ports.ModelSettingsStorage.
func (f *File) GetModel(ctx context.Context, model ids.ModelConfigID) (*entities.ModelSettings, error) {
	ctx, span := f.trace.Start(ctx, "File.GetModel")
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

	cfg, exists := storage[model.ID().String()]
	if !exists {
		return nil, ports.ErrNotFound
	}

	opts := []entities.NewModelSettingsOption{
		entities.WithSystemMessage(cfg.SystemMessage),
	}
	if cfg.Temperature != nil {
		opts = append(opts, entities.WithTemperature(*cfg.Temperature))
	}
	if cfg.TopP != nil {
		opts = append(opts, entities.WithTopP(*cfg.TopP))
	}
	if cfg.StopWords != nil {
		opts = append(opts, entities.WithStopWords(cfg.StopWords))
	}

	return entities.NewModelSettings(model, cfg.Model, opts...)
}

// ListModels implements ports.ModelSettingsStorage.
func (f *File) ListModels(ctx context.Context, user ids.UserID) ([]*entities.ModelSettings, error) {
	ctx, span := f.trace.Start(ctx, "File.ListModels")
	defer span.End()

	f.mu.Lock()
	defer f.mu.Unlock()

	data, err := os.ReadFile(f.path)
	if os.IsNotExist(err) {
		return nil, nil

	} else if err != nil {
		return nil, fmt.Errorf("failed to read token storage file: %w", err)
	}

	if len(data) == 0 {
		return nil, nil
	}

	var storage storageSchema
	if err := yaml.Unmarshal(data, &storage); err != nil {
		return nil, fmt.Errorf("failed to parse token storage file: %w", err)
	}

	configs := make([]*entities.ModelSettings, 0, len(storage))
	for name, info := range storage {
		id, err := ids.NewModelConfigIDFromString(name)
		if err != nil {
			return nil, fmt.Errorf("failed to create model config ID: %w", err)
		}

		opts := []entities.NewModelSettingsOption{
			entities.WithSystemMessage(info.SystemMessage),
		}
		if info.Temperature != nil {
			opts = append(opts, entities.WithTemperature(*info.Temperature))
		}
		if info.TopP != nil {
			opts = append(opts, entities.WithTopP(*info.TopP))
		}
		if info.StopWords != nil {
			opts = append(opts, entities.WithStopWords(info.StopWords))
		}

		cfg, err := entities.NewModelSettings(id, info.Model, opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create model settings for %v: %w", name, err)
		}

		configs = append(configs, cfg)
	}

	return configs, nil
}

// SaveModel implements ports.ModelSettingsStorage.
func (f *File) SaveModel(ctx context.Context, model entities.ModelSettingsReadOnly) error {
	ctx, span := f.trace.Start(ctx, "File.SaveModel")
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

	var temp *float32
	if t := model.Temperature(); t >= 0 {
		temp = ptr(t)
	}
	var topP *float32
	if t := model.TopP(); t >= 0 {
		topP = ptr(t)
	}

	storage[model.ID().ID().String()] = modelConfig{
		Model:         model.Model(),
		SystemMessage: model.SystemMessage(),
		Temperature:   temp,
		TopP:          topP,
		StopWords:     model.StopWords(),
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

type storageSchema map[configID]modelConfig

type configID = string

type modelConfig struct {
	Model string `yaml:"model"`

	SystemMessage string   `yaml:"system_message"`
	Temperature   *float32 `yaml:"temperature,omitempty"`
	TopP          *float32 `yaml:"top_p,omitempty"`
	StopWords     []string `yaml:"stop_words,omitempty"`
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func ptr[T any](v T) *T { return &v }

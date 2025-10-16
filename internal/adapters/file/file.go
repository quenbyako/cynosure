package file

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"sync"
	"time"

	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v3"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/entities"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/ports"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/ids"
	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/tools"
)

const pkgName = "internal/adapters/model-settings/file"

type File struct {
	mu   sync.Mutex
	path string

	trace trace.Tracer
}

var _ ports.ModelSettingsStorageFactory = (*File)(nil)
var _ ports.AccountStorageFactory = (*File)(nil)
var _ ports.ServerStorageFactory = (*File)(nil)

func (f *File) ServerStorage() ports.ServerStorage               { return f }
func (f *File) AccountStorage() ports.AccountStorage             { return f }
func (f *File) ModelSettingsStorage() ports.ModelSettingsStorage { return f }

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

	delete(storage.Models, model.ID().String())

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

	cfg, exists := storage.Models[model.ID().String()]
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

	configs := make([]*entities.ModelSettings, 0, len(storage.Models))
	for name, info := range storage.Models {
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

	storage.Models[model.ID().ID().String()] = modelConfig{
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

func (f *File) DeleteAccount(ctx context.Context, account ids.AccountID) error {
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

	// Check if user exists in storage
	userIDStr := account.User().ID().String()
	userTokens, exists := storage.Accounts[userIDStr]
	if !exists {
		return ports.ErrNotFound
	}

	serverAccounts, exists := userTokens[account.Server().ID().String()]
	if !exists {
		return ports.ErrNotFound
	}

	if _, exists := serverAccounts[account.ID().String()]; !exists {
		return ports.ErrNotFound
	}

	delete(serverAccounts, account.ID().String())

	if len(serverAccounts) == 0 {
		delete(userTokens, account.Server().ID().String())
	}

	if len(userTokens) == 0 {
		delete(storage.Accounts, userIDStr)
	}

	newData, err := yaml.Marshal(storage)
	if err != nil {
		return fmt.Errorf("failed to marshal updated token storage: %w", err)
	}

	if err := os.WriteFile(f.path, newData, 0644); err != nil {
		return fmt.Errorf("failed to write updated token storage file: %w", err)
	}

	return nil
}

func (f *File) GetAccount(ctx context.Context, account ids.AccountID) (*entities.Account, error) {
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

	// Check if user exists in storage
	userTokens, exists := storage.Accounts[account.User().ID().String()]
	if !exists {
		return nil, ports.ErrNotFound
	}

	serverAccounts, exists := userTokens[account.Server().ID().String()]
	if !exists {
		return nil, ports.ErrNotFound
	}

	token, exists := serverAccounts[account.ID().String()]
	if !exists {
		return nil, ports.ErrNotFound
	}

	registeredTools := make([]tools.ToolInfo, len(token.Tools))
	for i, t := range token.Tools {
		registeredTools[i], err = tools.NewToolInfo(t.Name, t.Desc, must(yamlAsJSON(t.Input)), must(yamlAsJSON(t.Output)))
		if err != nil {
			return nil, fmt.Errorf("failed to create tool info for %q: %w", t.Name, err)
		}
	}

	var opts []entities.NewAccountOption
	if token.Token != nil {
		opts = append(opts, entities.WithAuthToken(token.Token))
	}

	return entities.NewAccount(account, token.Name, token.Desc, registeredTools, opts...)
}

func (f *File) GetAccountsBatch(ctx context.Context, accounts []ids.AccountID) ([]*entities.Account, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	data, err := os.ReadFile(f.path)
	if os.IsNotExist(err) {
		return nil, &fs.PathError{Op: "read", Path: f.path, Err: ports.ErrNotFound}
	} else if err != nil {
		return nil, fmt.Errorf("failed to read token storage file: %w", err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("empty file: %w", ports.ErrNotFound)
	}

	var storage storageSchema
	if err := yaml.Unmarshal(data, &storage); err != nil {
		return nil, fmt.Errorf("failed to parse token storage file: %w", err)
	}

	var res []*entities.Account
	for _, account := range accounts {
		// Check if user exists in storage
		userTokens, exists := storage.Accounts[account.User().ID().String()]
		if !exists {
			continue
		}

		serverAccounts, exists := userTokens[account.Server().ID().String()]
		if !exists {
			continue
		}

		token, exists := serverAccounts[account.ID().String()]
		if !exists {
			continue
		}

		registeredTools := make([]tools.ToolInfo, len(token.Tools))
		for i, t := range token.Tools {
			registeredTools[i], err = tools.NewToolInfo(t.Name, t.Desc, must(yamlAsJSON(t.Input)), must(yamlAsJSON(t.Output)))
			if err != nil {
				return nil, fmt.Errorf("failed to create tool info for %q: %w", t.Name, err)
			}
		}
		var opts []entities.NewAccountOption
		if token.Token != nil {
			opts = append(opts, entities.WithAuthToken(token.Token))
		}

		acc, err := entities.NewAccount(account, token.Name, token.Desc, registeredTools, opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create account for %q: %w", account.ID(), err)
		}

		res = append(res, acc)
	}

	return res, nil
}

func (f *File) SaveAccount(ctx context.Context, info entities.AccountReadOnly) error {
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

	user := info.ID().User().ID().String()
	if storage.Accounts[user] == nil {
		storage.Accounts[user] = make(userServers)
	}

	server := info.ID().Server().ID().String()
	if storage.Accounts[user][server] == nil {
		storage.Accounts[user][server] = make(toolAccounts)
	}

	tools := info.Tools()
	toolsRaw := make([]tool, len(tools))
	for i, t := range tools {
		if !t.Valid() {
			return fmt.Errorf("invalid tool info for %q, index %v", t.Name(), i)
		}

		toolsRaw[i] = tool{
			Name:   t.Name(),
			Desc:   t.Desc(),
			Input:  must(jsonAsYaml(t.ParamsSchema())),
			Output: must(jsonAsYaml(t.ResponseSchema())),
		}
	}

	storage.Accounts[user][server][info.ID().ID().String()] = accountInfo{
		Token: info.Token(),
		Name:  info.Name(),
		Desc:  info.Description(),
		Tools: toolsRaw,
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

func (f *File) ListAccounts(ctx context.Context, user ids.UserID) ([]ids.AccountID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	data, err := os.ReadFile(f.path)
	if os.IsNotExist(err) {
		return []ids.AccountID{}, nil

	} else if err != nil {
		return nil, fmt.Errorf("failed to read token storage file: %w", err)
	}

	if len(data) == 0 {
		return []ids.AccountID{}, nil
	}

	var storage storageSchema
	if err := yaml.Unmarshal(data, &storage); err != nil {
		return nil, fmt.Errorf("failed to parse token storage file: %w", err)
	}

	// Check if user exists in storage
	userServers, exists := storage.Accounts[user.ID().String()]
	if !exists {
		return []ids.AccountID{}, nil
	}

	accountIDs := make([]ids.AccountID, 0, len(userServers))
	for serverID, serverAccounts := range userServers {
		server, err := ids.NewServerIDFromString(serverID)
		if err != nil {
			return nil, fmt.Errorf("failed to parse server ID: %w", err)
		}

		for accountID := range serverAccounts {
			id, err := ids.NewAccountIDFromString(user, server, accountID)
			if err != nil {
				return nil, fmt.Errorf("failed to parse account ID: %w", err)
			}

			accountIDs = append(accountIDs, id)
		}
	}

	return accountIDs, nil
}

func (f *File) AddServer(ctx context.Context, server ids.ServerID, info ports.ServerInfo) error {
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

	storage.Servers[server.ID().String()] = serverInfo{
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

func (f *File) GetServerInfo(ctx context.Context, server ids.ServerID) (*ports.ServerInfo, error) {
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

	cfg, exists := storage.Servers[server.ID().String()]
	if !exists {
		return nil, ports.ErrNotFound
	}

	return &ports.ServerInfo{
		SSELink:          must(url.Parse(cfg.Link)),
		AuthConfig:       cfg.Config,
		ConfigExpiration: cfg.Expiration,
	}, nil
}

func (f *File) ListServers(ctx context.Context, limit uint, token string) (m map[ids.ServerID]ports.ServerInfo, nextToken string, err error) {
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

	servers := make(map[ids.ServerID]ports.ServerInfo, len(storage.Servers))
	for name, info := range storage.Servers {
		servers[must(ids.NewServerIDFromString(name))] = ports.ServerInfo{
			SSELink:          must(url.Parse(info.Link)),
			AuthConfig:       info.Config,
			ConfigExpiration: info.Expiration,
		}
	}

	return servers, "", nil
}

func (f *File) LookupByURL(ctx context.Context, url *url.URL) (ids.ServerID, ports.ServerInfo, error) {
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

	for name, info := range storage.Servers {
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

type storageSchema struct {
	Models   map[configID]modelConfig `yaml:"models"`
	Accounts map[userID]userServers   `yaml:"accounts"`
	Servers  map[serverID]serverInfo  `yaml:"servers"`
}

type configID = string

type modelConfig struct {
	Model string `yaml:"model"`

	SystemMessage string   `yaml:"system_message"`
	Temperature   *float32 `yaml:"temperature,omitempty"`
	TopP          *float32 `yaml:"top_p,omitempty"`
	StopWords     []string `yaml:"stop_words,omitempty"`
}

// userTools holds all tokens for a single user, organized by tool.
type userServers map[serverID]toolAccounts

// toolAccounts holds all tokens for a specific tool, organized by account.
type toolAccounts map[accountID]accountInfo

type accountInfo struct {
	Name  string        `yaml:"name"`
	Desc  string        `yaml:"desc"`
	Token *oauth2.Token `yaml:"token"`
	Tools []tool        `yaml:"tools"`
}

type tool struct {
	Name   string    `yaml:"name"`
	Desc   string    `yaml:"desc"`
	Input  yaml.Node `yaml:"input"`
	Output yaml.Node `yaml:"output"`
}

type accountID = string
type userID = string
type serverID = string

type serverInfo struct {
	Link       string         `yaml:"url"`
	Config     *oauth2.Config `yaml:"config"`
	Expiration time.Time      `yaml:"expiration,omitempty"`
}

func jsonAsYaml(in json.RawMessage) (yaml.Node, error) {
	var middle any
	if err := json.Unmarshal(in, &middle); err != nil {
		return yaml.Node{}, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	var out yaml.Node
	if err := out.Encode(middle); err != nil {
		return yaml.Node{}, fmt.Errorf("failed to encode YAML: %w", err)
	}

	return out, nil
}

func yamlAsJSON(node yaml.Node) (json.RawMessage, error) {
	var middle any
	if err := (&node).Decode(&middle); err != nil {
		return nil, fmt.Errorf("failed to decode YAML: %w", err)
	}

	return json.Marshal(middle)
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func ptr[T any](v T) *T { return &v }

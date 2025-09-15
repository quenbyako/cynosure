package file

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sync"

	"golang.org/x/oauth2"
	"gopkg.in/yaml.v3"

	"tg-helper/internal/domains/components/ids"
	"tg-helper/internal/domains/components/tools"
	"tg-helper/internal/domains/entities"
	"tg-helper/internal/domains/ports"
)

type FileAccountStorage struct {
	mu   sync.Mutex
	path string
}

var _ ports.AccountStorage = (*FileAccountStorage)(nil)

func NewFileTokenStorage(path string) *FileAccountStorage {
	return &FileAccountStorage{
		path: path,
	}
}

func (f *FileAccountStorage) DeleteAccount(ctx context.Context, account ids.AccountID) error {
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
	userTokens, exists := storage[userIDStr]
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
		delete(storage, userIDStr)
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

func (f *FileAccountStorage) GetAccount(ctx context.Context, account ids.AccountID) (*entities.Account, error) {
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
	userTokens, exists := storage[account.User().ID().String()]
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

func (f *FileAccountStorage) GetAccountsBatch(ctx context.Context, accounts []ids.AccountID) ([]*entities.Account, error) {
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
		userTokens, exists := storage[account.User().ID().String()]
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

func (f *FileAccountStorage) SaveAccount(ctx context.Context, info entities.AccountReadOnly) error {
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
	if storage[user] == nil {
		storage[user] = make(userServers)
	}

	server := info.ID().Server().ID().String()
	if storage[user][server] == nil {
		storage[user][server] = make(toolAccounts)
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

	storage[user][server][info.ID().ID().String()] = accountInfo{
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

func (f *FileAccountStorage) ListAccounts(ctx context.Context, user ids.UserID) ([]ids.AccountID, error) {
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
	userServers, exists := storage[user.ID().String()]
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

// storageSchema defines the structure for storing all user tokens.
// It's a map where the key is the userID.
type storageSchema map[userID]userServers

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

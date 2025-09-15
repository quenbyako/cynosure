package chat

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"tg-helper/internal/domains/components/ids"
	"tg-helper/internal/domains/components/messages"
	"tg-helper/internal/domains/components/tools"
	"tg-helper/internal/domains/entities"
	"tg-helper/internal/domains/ports"
)

type relevantAccount struct {
	chosenTools map[string]struct{}
	account     *entities.Account
}

type Chat struct {
	thread        *entities.ChatHistory
	reversedTools map[string]tools.RawToolInfo

	storage  ports.StorageRepository
	tools    ports.ToolManager
	accounts ports.AccountStorage
}

func New(
	ctx context.Context,
	storage ports.StorageRepository,
	tools ports.ToolManager,
	accounts ports.AccountStorage,
	user ids.UserID,
	threadID string,
) (*Chat, error) {
	if storage == nil {
		return nil, fmt.Errorf("storage is nil")
	}
	if tools == nil {
		return nil, fmt.Errorf("tools is nil")
	}
	if accounts == nil {
		return nil, fmt.Errorf("accounts is nil")
	}

	if !user.Valid() {
		return nil, fmt.Errorf("user is invalid")
	}

	thread, err := storage.GetThread(ctx, user, threadID)
	if err != nil {
		return nil, fmt.Errorf("getting thread: %w", err)
	}

	reversedTools, err := pullToolsAndAccounts(ctx, tools, accounts, thread)
	if err != nil {
		return nil, fmt.Errorf("retrieving relevant tools and accounts: %w", err)
	}

	return &Chat{
		thread:        thread,
		reversedTools: reversedTools,

		storage:  storage,
		tools:    tools,
		accounts: accounts,
	}, nil
}

func CreateChatAggregate(
	ctx context.Context,
	storage ports.StorageRepository,
	tools ports.ToolManager,
	accounts ports.AccountStorage,
	userID ids.UserID,
	threadID string,
	messages []messages.Message,
) (*Chat, error) {
	thread, err := entities.NewChatHistory(userID, threadID, messages)
	if err != nil {
		return nil, fmt.Errorf("creating chat history: %w", err)
	}

	if err := storage.CreateThread(ctx, thread); err != nil {
		return nil, fmt.Errorf("creating thread in storage: %w", err)
	}

	reversedTools, err := pullToolsAndAccounts(ctx, tools, accounts, thread)
	if err != nil {
		return nil, fmt.Errorf("retrieving relevant tools and accounts: %w", err)
	}

	return &Chat{
		thread:        thread,
		reversedTools: reversedTools,

		storage:  storage,
		tools:    tools,
		accounts: accounts,
	}, nil
}

func (c *Chat) RelevantTools() []tools.RawToolInfo {
	return slices.Collect(maps.Values(c.reversedTools))
}

func (c *Chat) ThreadID() string             { return c.thread.ThreadID() }
func (c *Chat) Messages() []messages.Message { return c.thread.Messages() }

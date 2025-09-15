package zep

import (
	"context"
	"errors"
	"fmt"

	"github.com/getzep/zep-go/v2"
	v2client "github.com/getzep/zep-go/v2/client"
	option "github.com/getzep/zep-go/v2/option"

	"tg-helper/internal/domains/components/ids"
	"tg-helper/internal/domains/components/messages"
	"tg-helper/internal/domains/entities"
	"tg-helper/internal/domains/ports"
)

type ZepStorage struct {
	client *v2client.Client
}

var _ ports.StorageRepository = (*ZepStorage)(nil)

type NewZepStorageOpt = option.RequestOption

func WithAPIKey(apiKey string) NewZepStorageOpt { return option.WithAPIKey(apiKey) }

func NewZepStorage(opts ...NewZepStorageOpt) (*ZepStorage, error) {
	client := v2client.NewClient(opts...)

	return &ZepStorage{
		client: client,
	}, nil
}

func (z *ZepStorage) CreateThread(ctx context.Context, thread entities.ChatHistoryReadOnly) error {
	_, err := z.client.Memory.AddSession(ctx, &zep.CreateSessionRequest{
		SessionID: thread.ThreadID(),
		UserID:    thread.User().ID().String(),
	})

	if err != nil {
		return fmt.Errorf("creating thread: %w", err)
	}

	messages := make([]*zep.Message, len(thread.Messages()))
	for i, msg := range thread.Messages() {
		content, err := messagesToZepContent(msg)
		if err != nil {
			return fmt.Errorf("converting %v message to Zep content: %w", i, err)
		}

		messages[i] = content
	}

	_, err = z.client.Memory.Add(ctx, thread.ThreadID(), &zep.AddMemoryRequest{
		Messages: messages,
	})
	if err != nil {
		return fmt.Errorf("saving thread: %w", err)
	}

	return nil
}

// GetThread implements adapters.StorageRepository.
func (z *ZepStorage) GetThread(ctx context.Context, user ids.UserID, threadID string) (*entities.ChatHistory, error) {
	thread, err := z.client.Memory.GetSessionMessages(ctx, threadID, &zep.MemoryGetSessionMessagesRequest{})
	if e := new(zep.NotFoundError); errors.As(err, &e) {
		return nil, ports.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting thread: %w", err)
	}

	// for some reason, if session is not found, it returns empty
	// session with TotalCount == 0, so we need to check it
	if thread.TotalCount == nil || (thread.TotalCount != nil && *thread.TotalCount == 0) {
		return nil, ports.ErrNotFound
	}

	var messages []messages.Message
	for i, msg := range thread.Messages {
		message, err := MessageFromZepContent(msg)
		if err != nil {
			return nil, fmt.Errorf("converting %v message to domain message: %w", i, err)
		}
		messages = append(messages, message)
	}

	return entities.NewChatHistory(user, threadID, messages)
}

// SaveThread implements adapters.StorageRepository.
func (z *ZepStorage) SaveThread(ctx context.Context, thread entities.ChatHistoryReadOnly) error {
	if thread.Synchronized() {
		return nil // nothing to do
	}

	newMessages, err := chatHistoryEvents(thread.PendingEvents())
	if err != nil {
		return fmt.Errorf("converting chat history events to Zep messages: %w", err)
	}

	_, err = z.client.Memory.Add(ctx, thread.ThreadID(), &zep.AddMemoryRequest{
		Messages: newMessages,
	})
	if err != nil {
		return fmt.Errorf("saving thread: %w", err)
	}

	return nil
}

func ptr[T any](v T) *T {
	return &v
}
